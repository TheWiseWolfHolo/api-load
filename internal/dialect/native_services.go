package dialect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

// NativeServiceEndpoint 描述已核实的文件与任务接口。未登记路径不转发。
type NativeServiceEndpoint struct {
	Service         string
	Path            string
	Collection      string
	ID              string
	Operation       execution.Operation
	CreatesResource bool
	ModelRequired   bool
}

func ClassifyNativeService(method, path string) (NativeServiceEndpoint, error) {
	if path == "/native/soniox/tts" && method == http.MethodPost {
		return NativeServiceEndpoint{Service: "soniox", Path: "/tts", Collection: "tts", Operation: execution.OperationResourceCreate, ModelRequired: true}, nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/native/"), "/")
	if !strings.HasPrefix(path, "/native/") || len(parts) < 3 || parts[1] != "v1" {
		return NativeServiceEndpoint{}, fmt.Errorf("unsupported native service path")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "%\\?#") {
			return NativeServiceEndpoint{}, fmt.Errorf("invalid native resource path")
		}
	}
	e := NativeServiceEndpoint{Service: parts[0], Path: "/" + strings.Join(parts[1:], "/")}
	rest := parts[2:]
	if e.Service != "mistral" && e.Service != "soniox" && e.Service != "siliconflow" {
		return e, fmt.Errorf("unsupported native service")
	}
	if e.Service == "siliconflow" {
		e.Collection = "video"
		if len(rest) == 2 && rest[0] == "video" && method == http.MethodPost {
			if rest[1] == "submit" {
				e.Operation = execution.OperationResourceCreate
				e.CreatesResource = true
				e.ModelRequired = true
				return e, nil
			}
			if rest[1] == "status" {
				e.Operation = execution.OperationResourceRead
				return e, nil
			}
		}
		return e, &RequestValidationError{Field: "path", Rule: "unsupported_native_operation"}
	}
	if rest[0] == "files" {
		e.Collection = "files"
		if len(rest) == 1 && method == http.MethodPost {
			e.Operation = execution.OperationResourceCreate
			e.CreatesResource = true
			return e, nil
		}
		if len(rest) >= 2 {
			e.ID = rest[1]
		}
		if len(rest) == 2 && method == http.MethodDelete {
			e.Operation = execution.OperationResourceDelete
			return e, nil
		}
		if method == http.MethodGet && (len(rest) == 2 || len(rest) == 3 && e.Service == "mistral" && (rest[2] == "content" || rest[2] == "url")) {
			e.Operation = execution.OperationResourceRead
			return e, nil
		}
	}
	if e.Service == "soniox" && rest[0] == "transcriptions" {
		e.Collection = "transcriptions"
		if len(rest) == 1 && method == http.MethodPost {
			e.Operation = execution.OperationResourceCreate
			e.CreatesResource = true
			e.ModelRequired = true
			return e, nil
		}
		if len(rest) >= 2 {
			e.ID = rest[1]
		}
		if len(rest) == 2 && method == http.MethodDelete {
			e.Operation = execution.OperationResourceDelete
			return e, nil
		}
		if method == http.MethodGet && (len(rest) == 2 || len(rest) == 3 && rest[2] == "transcript") {
			e.Operation = execution.OperationResourceRead
			return e, nil
		}
	}
	if e.Service == "mistral" && len(rest) >= 2 && rest[0] == "batch" && rest[1] == "jobs" {
		e.Collection = "batch/jobs"
		if len(rest) == 2 && method == http.MethodPost {
			e.Operation = execution.OperationResourceCreate
			e.CreatesResource = true
			e.ModelRequired = true
			return e, nil
		}
		if len(rest) >= 3 {
			e.ID = rest[2]
		}
		if len(rest) == 3 && method == http.MethodGet {
			e.Operation = execution.OperationResourceRead
			return e, nil
		}
		if len(rest) == 3 && method == http.MethodDelete {
			e.Operation = execution.OperationResourceDelete
			return e, nil
		}
		if len(rest) == 4 && rest[3] == "cancel" && method == http.MethodPost {
			e.Operation = execution.OperationResourceCancel
			return e, nil
		}
	}
	return e, &RequestValidationError{Field: "path", Rule: "unsupported_native_operation"}
}

func NativeResourceKey(service, collection, id string) string {
	return "native/" + service + "/" + collection + "/" + id
}

type NativeServices struct{}

func NewNativeServices() *NativeServices            { return &NativeServices{} }
func (*NativeServices) Protocol() protocol.Protocol { return protocol.NativeServices }
func (*NativeServices) InspectRequest(request *ParsedRequest) (RequestMetadata, error) {
	e, err := ClassifyNativeService(request.Method, request.Path)
	if err != nil {
		return RequestMetadata{}, err
	}
	m := RequestMetadata{Operation: e.Operation, RouteRequirement: execution.RouteRequirementNative}
	if e.ID != "" {
		m.ResourceReferences = []string{NativeResourceKey(e.Service, e.Collection, e.ID)}
	}
	if e.Service == "siliconflow" && e.Operation == execution.OperationResourceRead {
		var body struct {
			ID string `json:"requestId"`
		}
		if json.Unmarshal(request.Body, &body) != nil || body.ID == "" {
			return m, &RequestValidationError{Field: "requestId", Rule: "required_resource_id"}
		}
		m.ResourceReferences = []string{NativeResourceKey(e.Service, e.Collection, body.ID)}
	}
	if e.Operation != execution.OperationResourceCreate {
		return m, nil
	}
	if e.Collection == "files" {
		media, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || media != "multipart/form-data" || params["boundary"] == "" {
			return m, &RequestValidationError{Field: "file", Rule: "multipart_required"}
		}
		reader := multipart.NewReader(bytes.NewReader(request.Body), params["boundary"])
		found := false
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return m, &RequestValidationError{Field: "file", Rule: "invalid_multipart"}
			}
			if part.FormName() == "file" {
				found = true
			}
			if _, err = io.Copy(io.Discard, part); err != nil {
				return m, err
			}
		}
		if !found {
			return m, &RequestValidationError{Field: "file", Rule: "required_file"}
		}
		return m, nil
	}
	m, err = inspectJSONRequestFields(request.Body, e.ModelRequired, false)
	if err != nil {
		return m, err
	}
	if m.Stream {
		return m, &RequestValidationError{Field: "stream", Rule: "streaming_unsupported"}
	}
	m.Operation = e.Operation
	m.ObserveUsage = true
	m.RouteRequirement = execution.RouteRequirementNative
	var body struct {
		FileID     string   `json:"file_id"`
		AudioURL   string   `json:"audio_url"`
		InputFiles []string `json:"input_files"`
	}
	if err = json.Unmarshal(request.Body, &body); err != nil {
		return m, err
	}
	if e.Service == "soniox" && e.Collection == "transcriptions" && (body.FileID == "") == (body.AudioURL == "") {
		return m, &RequestValidationError{Field: "file_id_audio_url", Rule: "exactly_one_required"}
	}
	if body.FileID != "" {
		m.ResourceReferences = append(m.ResourceReferences, NativeResourceKey(e.Service, "files", body.FileID))
	}
	for _, id := range body.InputFiles {
		if id == "" {
			return m, &RequestValidationError{Field: "input_files", Rule: "required_resource_id"}
		}
		m.ResourceReferences = append(m.ResourceReferences, NativeResourceKey(e.Service, "files", id))
	}
	return m, nil
}

func (d *NativeServices) RewriteRequestModel(request *ParsedRequest, model string) (*ParsedRequest, error) {
	m, err := d.InspectRequest(request)
	if err != nil {
		return nil, err
	}
	if m.Model == nil {
		return cloneParsedRequest(request)
	}
	return rewriteJSONRequestModel(request, model, string(d.Protocol()))
}
func (*NativeServices) RewriteResponseModel(body []byte, _ string) ([]byte, error) {
	return bytes.Clone(body), nil
}
