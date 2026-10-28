package gateway

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/execution"
	"gpt-load/internal/health"
	"gpt-load/internal/outboundproxy"
	"gpt-load/internal/platform/httpclient"
	"gpt-load/internal/protocol"
	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
	"gpt-load/internal/telemetry"
	"gpt-load/internal/usage"
)

var errTransparentIdle = errors.New("transparent response idle timeout")
var errTransparentCredentialEcho = errors.New("upstream response contains credential material")

func transparentGatewayKey(request *http.Request) string {
	fields := strings.Fields(request.Header.Get("Authorization"))
	if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
		return fields[1]
	}
	for _, name := range []string{"X-Api-Key", "X-Goog-Api-Key"} {
		if value := strings.TrimSpace(request.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func transparentHeaderContainsSecret(value string, secrets []string) bool {
	decoded, _ := url.QueryUnescape(value)
	for _, secret := range secrets {
		if secret != "" && (strings.Contains(value, secret) || strings.Contains(decoded, secret)) {
			return true
		}
	}
	return false
}

func transparentHeadersContainSecret(headers http.Header, secrets []string) bool {
	for name, values := range headers {
		if transparentHeaderContainsSecret(name, secrets) {
			return true
		}
		for _, value := range values {
			if transparentHeaderContainsSecret(value, secrets) {
				return true
			}
		}
	}
	return false
}

// Keep every business query fragment byte-for-byte, except a duplicate gateway credential.
func transparentQuery(raw, token string) string {
	segments := strings.Split(raw, "&")
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		name, value, _ := strings.Cut(segment, "=")
		decodedName, nerr := url.QueryUnescape(name)
		decodedValue, verr := url.QueryUnescape(value)
		if nerr == nil && verr == nil && decodedName == "key" && decodedValue == token {
			continue
		}
		kept = append(kept, segment)
	}
	return strings.Join(kept, "&")
}

func transparentPath(request *http.Request) (uint, string, string, error) {
	if request.URL.IsAbs() || request.URL.Host != "" {
		return 0, "", "", errors.New("absolute proxy URL is unsupported")
	}
	parts := strings.SplitN(request.URL.EscapedPath(), "/", 4)
	if len(parts) != 4 || parts[1] != "proxy" {
		return 0, "", "", errors.New("invalid proxy path")
	}
	id, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != parts[2] {
		return 0, "", "", errors.New("invalid source ID")
	}
	raw := "/" + parts[3]
	decoded, err := url.PathUnescape(raw)
	if err != nil || strings.HasPrefix(decoded, "//") || strings.Contains(decoded, "://") || strings.ContainsAny(decoded, "%\\\r\n\x00") {
		return 0, "", "", errors.New("invalid proxy path")
	}
	// Escaped separators and dot segments must not bypass an exact path grant.
	if len(strings.Split(raw, "/")) != len(strings.Split(decoded, "/")) {
		return 0, "", "", errors.New("encoded path separator")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return 0, "", "", errors.New("path traversal")
		}
	}
	return uint(id), decoded, raw, nil
}

func transparentTarget(base, escapedPath, rawQuery string) (*url.URL, error) {
	target, err := url.Parse(base)
	if err != nil || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || (target.Scheme != "https" && target.Scheme != "http") {
		return nil, errors.New("invalid configured target")
	}
	raw := strings.TrimRight(target.EscapedPath(), "/") + escapedPath
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return nil, err
	}
	target.Path, target.RawPath, target.RawQuery = decoded, raw, rawQuery
	return target, nil
}

func (handler *Handler) transparentClient(revision uint64, proxy outboundproxy.Effective, timeouts state.TimeoutConfig) (*http.Client, error) {
	handler.transparentClientsMu.Lock()
	defer handler.transparentClientsMu.Unlock()
	if handler.transparentClients == nil || handler.transparentRevision != revision {
		if handler.transparentClients != nil {
			handler.transparentClients.CloseIdleConnections()
		}
		handler.transparentClients = httpclient.NewHTTPClientManager()
		handler.transparentRevision = revision
	}
	return handler.transparentClients.GetClientForOutboundProxy(&httpclient.Config{ConnectTimeout: timeouts.FirstByte, ResponseHeaderTimeout: timeouts.FirstByte, TLSHandshakeTimeout: timeouts.FirstByte, IdleConnTimeout: 90 * time.Second, MaxIdleConns: 100, MaxIdleConnsPerHost: 20, DisableCompression: true, DisableRedirects: true, ForceAttemptHTTP2: true}, proxy)
}

type transparentTransportFunc func(*http.Request) (*http.Response, error)

func (f transparentTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type transparentResponseBody struct {
	io.ReadCloser
	onRead func(int, error) error
}

type transparentDownstreamWriter struct {
	http.ResponseWriter
	control *streamWriteController
}

func (w transparentDownstreamWriter) Write(body []byte) (int, error) { return w.control.write(body) }
func (w transparentDownstreamWriter) WriteHeader(status int) {
	if w.control.writeHeader(status) != nil {
		panic(http.ErrAbortHandler)
	}
}
func (w transparentDownstreamWriter) FlushError() error           { return w.control.flush() }
func (w transparentDownstreamWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (body transparentResponseBody) Read(value []byte) (int, error) {
	n, err := body.ReadCloser.Read(value)
	return n, body.onRead(n, err)
}

func (handler *Handler) handleTransparent(c *gin.Context, snapshot *state.ConfigSnapshot, key state.AccessKeyView, recorder *requestRecorder) {
	recorder.setOperation(execution.OperationTransparentRequest)
	recorder.setUsageApplicable(true)
	fail := func(status int, code, message string) {
		handler.completeReason(c, recorder, reason{status, code, message})
	}
	if c.Request.Method == http.MethodConnect || c.Request.Method == http.MethodTrace || websocketIntent(c.Request) || c.Request.Header.Get("Upgrade") != "" {
		fail(405, "transparent_method_unsupported", "This transparent endpoint does not support tunneling or protocol upgrades.")
		return
	}
	groupID, path, escapedPath, err := transparentPath(c.Request)
	if err != nil {
		fail(400, "transparent_path_invalid", "The proxy path must stay within its configured source.")
		return
	}
	if !key.Filters.AllowsTransparent(groupID, c.Request.Method, path) {
		fail(403, "transparent_scope_denied", "The access key does not allow this source, method and path.")
		return
	}
	if len(key.CostLimitRules) > 0 {
		fail(400, "transparent_cost_limit_unsupported", "Opaque responses cannot enforce token-cost limits. Use a separately scoped access key.")
		return
	}
	if !snapshot.RequestRedaction.Empty() || snapshot.RequestAudit.Enabled {
		fail(400, "transparent_content_policy_unsupported", "Transparent forwarding cannot apply configured content inspection or redaction.")
		return
	}
	group, exists := snapshot.Groups[groupID]
	if !exists || !group.TransparentProxy.Enabled {
		fail(404, "transparent_source_unavailable", "The transparent source is unavailable.")
		return
	}
	policy := group.TransparentProxy
	target, err := transparentTarget(policy.BaseURL, escapedPath, c.Request.URL.RawQuery)
	if err != nil {
		fail(503, "transparent_target_invalid", "The transparent source target is invalid.")
		return
	}
	if c.Request.ContentLength > handler.RequestBodyLimit() {
		handler.completeReason(c, recorder, reasonRequestTooLarge)
		return
	}
	refs := handler.registry.CaptureActiveCredentialRefs([]uint{groupID})
	allowed := make(map[uint]state.CredentialRef)
	for _, ref := range refs {
		if policy.Strategy != "fixed" || ref.ID == policy.FixedCredentialID {
			allowed[ref.ID] = ref
		}
	}
	if len(allowed) == 0 {
		fail(503, "transparent_credential_unavailable", "No permitted credential is currently available; fixed sources do not switch credentials.")
		return
	}
	queryKey := key
	queryKey.Filters.GroupsRestricted = true
	queryKey.Filters.Groups = map[uint]struct{}{groupID: {}}
	iterator := scheduler.New(snapshot, handler.registry, scheduler.Query{ClientProtocol: protocol.TransparentHTTP, Operation: execution.OperationTransparentRequest, AccessKey: queryKey, AllowedCredentialRefs: allowed})
	selection, err := iterator.Next()
	if err != nil {
		fail(503, "transparent_credential_unavailable", "No permitted credential is currently available; fixed sources do not switch credentials.")
		return
	}
	ref := allowed[selection.CredentialID]
	encrypted, active := handler.registry.ActiveEncryptedCredentialDataIfMatch(ref)
	if !active {
		handler.completeConfigurationChanged(c, recorder)
		return
	}
	plaintext, err := handler.encryption.Decrypt(encrypted)
	if err != nil {
		fail(503, "transparent_credential_invalid", "The selected credential could not be prepared.")
		return
	}
	credential, err := normalizeChannelCredential(handler.channels, handler.subscriptions, selection.ChannelID, selection.Group.ConnectionType, plaintext)
	plaintext = ""
	if err != nil || credential.apiKey == "" {
		fail(503, "transparent_credential_invalid", "The selected source requires a static API key.")
		return
	}
	proxy, _, err := resolveAttemptProxy(handler.encryption, selection.Group.Proxy, ref)
	if err != nil {
		fail(503, "transparent_proxy_invalid", "The outbound proxy could not be prepared.")
		return
	}
	client, err := handler.transparentClient(snapshot.Revision, proxy, selection.Group.Timeouts)
	if err != nil {
		fail(503, "transparent_proxy_invalid", "The outbound proxy could not be prepared.")
		return
	}
	release, failure := handler.acquireGroupConcurrency(groupID)
	if failure != nil {
		handler.completeReason(c, recorder, *failure)
		return
	}
	defer release()
	ctx, cancel := context.WithCancelCause(c.Request.Context())
	defer cancel(nil)
	if selection.Group.Timeouts.Request > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, selection.Group.Timeouts.Request)
		defer stop()
	}
	request := c.Request.Clone(ctx)
	request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, handler.RequestBodyLimit())
	recorder.startTiming()
	started := handler.now()
	recorder.freezeNextAttemptPricing(frozenAttemptPricing{groupID: selection.GroupID, channelID: string(selection.ChannelID), applicable: true, metadataSet: true})
	result := UpstreamResult{DispatchState: execution.DispatchMaybeSent, UpstreamProtocol: protocol.TransparentHTTP, Usage: usage.Result{State: usage.StateMissing}}
	decisionContext := health.DecisionContext{ErrorRules: selection.Group.ErrorRules, DefaultRateLimitCooldown: fixedCooldown, Method: request.Method, Operation: execution.OperationTransparentRequest}
	var responseDecision *health.Decision
	var readErr error
	var idle *time.Timer
	defer func() {
		if idle != nil {
			idle.Stop()
		}
		panicValue := recover()
		result.Committed = c.Writer.Written()
		if result.StatusCode == 0 {
			result.StatusCode = c.Writer.Status()
		}
		if readErr != nil || panicValue != nil || ctx.Err() != nil {
			result.Stream.EndReason = StreamEndUpstreamTerminated
			if errors.Is(context.Cause(ctx), errTransparentIdle) {
				result.Stream.EndReason = StreamEndIdleTimeout
			} else if c.Request.Context().Err() != nil {
				result.Stream.EndReason = StreamEndClientCanceled
			} else if panicValue != nil && readErr == nil {
				result.Stream.EndReason = StreamEndDownstreamWriteFailure
			}
			result.Stream.ErrorSummary = "Transparent response did not finish."
		}
		decision := judgeUpstreamResult(result, handler.now(), decisionContext)
		if responseDecision != nil && result.Err == nil && readErr == nil && panicValue == nil {
			decision = *responseDecision
		}
		decision.Retry = health.RetryNone
		index := recorder.recordAttempt(selection, credential.secrets, result, decision, started, handler.now())
		if result.Stream.EndReason != StreamEndNone {
			recorder.completeStream(result, "", index)
		} else {
			recorder.completeResponse(result, decision, "", index)
			if result.StatusCode >= 300 && result.StatusCode < 400 {
				recorder.outcome.status = telemetry.RequestStatusSuccess
			}
		}
		handler.applyGroupDecisionEffect(selection.Group, ref, 0, decision, result.StatusCode, handler.now(), "")
		if result.StatusCode >= 200 && result.StatusCode < 400 && result.Err == nil && readErr == nil && panicValue == nil {
			handler.recordCredentialSuccess(ref, handler.now())
		}
		if panicValue != nil {
			panic(panicValue)
		}
	}()
	proxyHandler := &httputil.ReverseProxy{
		FlushInterval: -1,
		ErrorLog:      log.New(io.Discard, "", 0),
		Rewrite: func(p *httputil.ProxyRequest) {
			p.Out.URL = target
			p.Out.Host = target.Host
			p.Out.GetBody = nil
			p.Out.Body = request.Body // A non-replayable reader also prevents empty idempotency-key POST retries.
			for _, name := range []string{"Authorization", "X-Api-Key", "X-Goog-Api-Key", "Proxy-Authorization", policy.AuthHeader} {
				p.Out.Header.Del(name)
			}
			for name, values := range p.Out.Header {
				for _, value := range values {
					if transparentHeaderContainsSecret(value, []string{transparentGatewayKey(c.Request), credential.apiKey}) {
						p.Out.Header.Del(name)
						break
					}
				}
			}
			p.Out.Header.Set(policy.AuthHeader, policy.AuthPrefix+credential.apiKey)
		},
		Transport: transparentTransportFunc(func(r *http.Request) (*http.Response, error) {
			// ReverseProxy installs a per-request trace that forwards 1xx headers before
			// ModifyResponse. Replace that hook so it cannot bypass credential checks.
			// Gin defers status writes; omit interim responses rather than change its
			// final-response state. The final headers/body remain opaque.
			trace := httptrace.ContextClientTrace(r.Context())
			trace.Got1xxResponse = func(_ int, header textproto.MIMEHeader) error {
				if transparentHeadersContainSecret(http.Header(header), append([]string{transparentGatewayKey(c.Request)}, credential.secrets...)) {
					return errTransparentCredentialEcho
				}
				return nil
			}
			response, err := client.Transport.RoundTrip(r)
			result.RequestWritten = true
			if err != nil {
				result.Err = err
				result.ErrorSummary = "Transparent upstream transport failed."
				return nil, err
			}
			result.StatusCode = response.StatusCode
			result.Header = response.Header.Clone()
			result.ResponseStarted = true
			result.UpstreamRequestID = response.Header.Get("X-Request-Id")
			if result.UpstreamRequestID == "" {
				result.UpstreamRequestID = response.Header.Get("Request-Id")
			}
			return response, nil
		}),
		ModifyResponse: func(response *http.Response) error {
			if response.StatusCode == http.StatusSwitchingProtocols {
				return errors.New("upstream protocol upgrades are unsupported")
			}
			secrets := append([]string{transparentGatewayKey(c.Request)}, credential.secrets...)
			if transparentHeadersContainSecret(response.Header, secrets) {
				result.Header = nil
				result.UpstreamRequestID = ""
				return errTransparentCredentialEcho
			}
			for _, name := range []string{"Authorization", "X-Api-Key", "X-Goog-Api-Key", policy.AuthHeader} {
				response.Header.Del(name)
			}
			// Judge only the observed HTTP status. Opaque error bodies cannot establish
			// invalid credentials or safe replay. Explicit administrator rules still apply.
			if response.StatusCode >= 400 {
				result.ExecutionError = &execution.ErrorEvidence{Kind: execution.ErrorKindHTTP, OriginHint: execution.ErrorOriginUpstream,
					ScopeHint: execution.ErrorScopeRequest, StatusCode: response.StatusCode, Code: "transparent_http_status",
					Summary: "The transparent upstream returned an HTTP error status.", ReplaySafety: execution.ReplaySafetyUnknown}
				if response.StatusCode == http.StatusTooManyRequests {
					result.ExecutionError.ScopeHint = execution.ErrorScopeCredential
				}
				decision := judgeUpstreamResult(result, handler.now(), decisionContext)
				responseDecision = &decision
			}
			recorder.setStream(strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream"))
			if selection.Group.Timeouts.StreamIdle > 0 {
				idle = time.AfterFunc(selection.Group.Timeouts.StreamIdle, func() { cancel(errTransparentIdle) })
			}
			response.Body = transparentResponseBody{ReadCloser: response.Body, onRead: func(n int, err error) error {
				if n > 0 {
					recorder.recordFirstResponse()
				}
				if idle != nil && n > 0 {
					idle.Reset(selection.Group.Timeouts.StreamIdle)
				}
				if err == io.EOF && transparentHeadersContainSecret(response.Trailer, secrets) {
					err = errTransparentCredentialEcho
				}
				if err != nil && err != io.EOF {
					readErr = err
				}
				return err
			}}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			result.Err = err
			result.ErrorSummary = "Transparent upstream transport failed."
			if errors.Is(err, errTransparentCredentialEcho) {
				result.StatusCode = 502
				handler.writeReason(c, reason{502, "transparent_credential_response_blocked", "The upstream response contained credential material and was not forwarded."})
				return
			}
			var oversized *http.MaxBytesError
			if errors.As(err, &oversized) {
				result.StatusCode = 413
				handler.writeReason(c, reasonRequestTooLarge)
				return
			}
			result.StatusCode = 502
			handler.writeReason(c, reason{502, "transparent_upstream_failed", "The upstream request failed; it was not retried."})
		},
	}
	writer := newStreamWriteController(c.Writer, handler.writeTimeout)
	defer writer.clear()
	proxyHandler.ServeHTTP(transparentDownstreamWriter{ResponseWriter: c.Writer, control: writer}, request)
}
