package bifrost

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/klauspost/compress/zstd"

	"gpt-load/internal/channel"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

func TestMistralNativeRoutesPreserveMethodPathAndCredential(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		operation execution.Operation
		method    string
		path      string
		body      string
		model     string
		wantPath  string
		wantBody  string
	}{
		{
			name: "ocr", operation: execution.OperationMistralOCR, method: http.MethodPost, path: "/v1/ocr",
			body:  `{"model":"public","document":{"type":"document_url","document_url":"https://example.com/a.pdf"}}`,
			model: "upstream-ocr", wantPath: "/v1/ocr", wantBody: `"model":"upstream-ocr"`,
		},
		{
			name: "voices", operation: execution.OperationMistralVoices, method: http.MethodGet, path: "/mistral/v1/audio/voices",
			wantPath: "/v1/audio/voices",
		},
		{
			name: "voices v2", operation: execution.OperationMistralVoices, method: http.MethodGet, path: "/mistral/v2/audio/voices",
			wantPath: "/v2/audio/voices",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				if request.Method != test.method || request.URL.Path != test.wantPath || request.URL.RawQuery != "tenant=a%2Bb" {
					t.Errorf("target = %s %s?%s", request.Method, request.URL.Path, request.URL.RawQuery)
				}
				if request.Header.Get("Authorization") != "Bearer "+testAPIKey || request.Header.Get("X-Api-Key") != "" {
					t.Error("credential headers were not replaced")
				}
				payload, err := io.ReadAll(request.Body)
				if err != nil {
					t.Error(err)
					return
				}
				if test.wantBody != "" && !bytes.Contains(payload, []byte(test.wantBody)) {
					t.Errorf("body = %s", payload)
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.Header().Set("X-Request-Id", "mistral-request")
				_, _ = io.WriteString(writer, `{"model":"upstream-ocr","ok":true}`)
			}))
			defer server.Close()

			spec := openAIEmbeddingsSpec(channel.Mistral, server.URL+"/v1", []byte(test.body))
			spec.ClientProtocol = protocol.Mistral
			spec.Operation = test.operation
			spec.Method = test.method
			spec.Path = test.path
			spec.RawQuery = "tenant=a%2Bb&api_key=injected"
			if test.model == "" {
				spec.ClientModel = ""
				spec.UpstreamModel = ""
			} else {
				spec.ClientModel = "public"
				spec.UpstreamModel = test.model
			}
			spec = freezeTestAttempt(spec)
			runtime := newProtocolTestRuntime(t, testRuntimeOptions{allowPrivateNetwork: true})
			result := runtime.Execute(context.Background(), spec)
			if err := result.Validate(); err != nil || result.Error != nil || result.StatusCode != http.StatusOK || calls.Load() != 1 {
				t.Fatalf("calls=%d result=%+v error=%+v contract=%v body=%s", calls.Load(), result, result.Error, err, result.Body)
			}
			if result.UpstreamProtocol != protocol.Mistral || result.UpstreamRequestID != "mistral-request" {
				t.Fatalf("protocol/request id = %s / %s", result.UpstreamProtocol, result.UpstreamRequestID)
			}
		})
	}
}

func TestNativeResourcePathAndMultipartPreserved(t *testing.T) {
	for _, channelID := range []channel.ID{channel.Mistral, channel.Soniox} {
		original := []byte("--native-boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"sample.txt\"\r\nContent-Type: text/plain\r\n\r\nsynthetic file\r\n--native-boundary--\r\n")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if r.URL.Path != "/v1/files" || string(body) != string(original) {
				t.Errorf("multipart changed: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"file-synthetic"}`)
		}))
		spec := openAIEmbeddingsSpec(channelID, server.URL+"/v1", original)
		spec.ClientProtocol = protocol.NativeServices
		spec.Operation = execution.OperationResourceCreate
		spec.Path = "/native/" + string(channelID) + "/v1/files"
		spec.Header.Set("Content-Type", "multipart/form-data; boundary=native-boundary")
		result := newProtocolTestRuntime(t, testRuntimeOptions{allowPrivateNetwork: true}).Execute(t.Context(), freezeTestAttempt(spec))
		server.Close()
		if result.Error != nil || result.StatusCode != 200 {
			t.Fatalf("result=%+v error=%+v", result, result.Error)
		}
	}
}

func TestNativeSonioxSpeechUsesSeparateTargetAndReturnsAudio(t *testing.T) {
	audio := []byte("RIFFsynthetic-audio-binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/tts" || r.Header.Get("Authorization") != "Bearer "+testAPIKey || !bytes.Contains(body, []byte(`"voice":"Synthetic"`)) {
			t.Error("speech protocol or credential changed")
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(audio)
	}))
	defer server.Close()
	spec := openAIEmbeddingsSpec(channel.Soniox, server.URL+"/v1", []byte(`{"model":"public","language":"en","voice":"Synthetic","audio_format":"wav","text":"synthetic"}`))
	spec.ClientProtocol = protocol.NativeServices
	spec.Operation = execution.OperationResourceCreate
	spec.Path = "/native/soniox/tts"
	spec.TargetConfig, _ = json.Marshal(map[string]string{"base_url": server.URL + "/v1", "tts_base_url": server.URL})
	result := newProtocolTestRuntime(t, testRuntimeOptions{allowPrivateNetwork: true}).Execute(t.Context(), freezeTestAttempt(spec))
	if result.Error != nil || !bytes.Equal(result.Body, audio) {
		t.Fatalf("speech result=%+v error=%+v", result, result.Error)
	}
}

func TestNativeSpeechDistinguishesPCMFromActualCompression(t *testing.T) {
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	_, _ = zw.Write([]byte("synthetic audio"))
	_ = zw.Close()
	zstdWriter, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	zstdBytes := zstdWriter.EncodeAll([]byte("synthetic audio"), nil)
	zstdWriter.Close()
	for _, provider := range []channel.ID{channel.Mistral, channel.Soniox} {
		for _, test := range []struct {
			name     string
			body     []byte
			rejected bool
		}{
			{"pcm-gzip-prefix", []byte{0x1f, 0x8b, 0x00, 0x01, 0x02, 0x03}, false},
			{"pcm-zstd-prefix", []byte{0x28, 0xb5, 0x2f, 0xfd, 0xff, 0x00}, false},
			{"actual-gzip", zipped.Bytes(), true}, {"actual-zstd", zstdBytes, true},
		} {
			t.Run(string(provider)+"/"+test.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "audio/pcm")
					_, _ = w.Write(test.body)
				}))
				defer server.Close()
				spec := openAIEmbeddingsSpec(provider, server.URL+"/v1", []byte(`{"model":"public","text":"synthetic","input":"synthetic","voice":"Synthetic"}`))
				spec.ClientProtocol = protocol.Mistral
				spec.Operation = execution.OperationMistralAudioSpeech
				spec.Path = "/v1/audio/speech"
				if provider == channel.Soniox {
					spec.ClientProtocol = protocol.NativeServices
					spec.Operation = execution.OperationResourceCreate
					spec.Path = "/native/soniox/tts"
					spec.TargetConfig, _ = json.Marshal(map[string]string{"base_url": server.URL + "/v1", "tts_base_url": server.URL})
				}
				result := newProtocolTestRuntime(t, testRuntimeOptions{allowPrivateNetwork: true}).Execute(t.Context(), freezeTestAttempt(spec))
				if test.rejected {
					if result.Error == nil {
						t.Fatal("compressed audio without encoding header was accepted")
					}
				} else if result.Error != nil || !bytes.Equal(result.Body, test.body) {
					t.Fatalf("raw PCM changed or failed: %+v", result.Error)
				}
			})
		}
	}
}
