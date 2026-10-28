package dialect

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

func TestMistralInspectsDocumentedModelPaths(t *testing.T) {
	t.Parallel()
	dialect := NewMistral()
	if dialect.Protocol() != protocol.Mistral {
		t.Fatalf("protocol = %s", dialect.Protocol())
	}
	tests := []struct {
		name      string
		method    string
		path      string
		rawQuery  string
		body      string
		operation execution.Operation
		model     string
		stream    bool
	}{
		{name: "ocr", method: http.MethodPost, path: "/v1/ocr", body: `{"model":"mistral-ocr-latest","document":{"type":"document_url"}}`, operation: execution.OperationMistralOCR, model: "mistral-ocr-latest"},
		{name: "fim", method: http.MethodPost, path: "/v1/fim/completions", body: `{"model":"codestral-latest","prompt":"fn "}`, operation: execution.OperationMistralFIM, model: "codestral-latest"},
		{name: "transcription", method: http.MethodPost, path: "/v1/audio/transcriptions", body: `{"model":"voxtral"}`, operation: execution.OperationMistralAudioTranscription, model: "voxtral"},
		{name: "speech", method: http.MethodPost, path: "/v1/audio/speech", body: `{"model":"voxtral-tts","input":"hello"}`, operation: execution.OperationMistralAudioSpeech, model: "voxtral-tts"},
		{name: "realtime", method: http.MethodGet, path: "/v1/audio/transcriptions/realtime", rawQuery: "model=voxtral-mini-transcribe-realtime-2602", operation: execution.OperationMistralRealtimeTranscription, model: "voxtral-mini-transcribe-realtime-2602"},
		{name: "voices", method: http.MethodGet, path: "/mistral/v1/audio/voices", operation: execution.OperationMistralVoices},
		{name: "voice", method: http.MethodGet, path: "/mistral/v1/audio/voices/530e2e20-58e2-45d8-b0a5-4594f4915944", operation: execution.OperationMistralVoices},
		{name: "voices v2", method: http.MethodGet, path: "/mistral/v2/audio/voices", operation: execution.OperationMistralVoices},
		{name: "moderation", method: http.MethodPost, path: "/v1/moderations", body: `{"model":"mistral-moderation-latest","input":"hello"}`, operation: execution.OperationMistralModeration, model: "mistral-moderation-latest"},
		{name: "chat moderation", method: http.MethodPost, path: "/v1/chat/moderations", body: `{"model":"mistral-moderation-latest","input":[]}`, operation: execution.OperationMistralChatModeration, model: "mistral-moderation-latest"},
		{name: "classification", method: http.MethodPost, path: "/v1/classifications", body: `{"model":"shieldstral","input":"hello"}`, operation: execution.OperationMistralClassification, model: "shieldstral"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			header := make(http.Header)
			if test.body != "" {
				header.Set("Content-Type", "application/json")
			}
			metadata, err := dialect.InspectRequest(&ParsedRequest{
				Method: test.method, Path: test.path, RawQuery: test.rawQuery, Header: header, Body: []byte(test.body),
			})
			if err != nil {
				t.Fatalf("InspectRequest() error = %v", err)
			}
			if metadata.Operation != test.operation || metadata.Stream != test.stream || metadata.RouteRequirement != execution.RouteRequirementNative {
				t.Fatalf("metadata = %+v", metadata)
			}
			if test.model == "" {
				if metadata.Model != nil {
					t.Fatalf("model = %q, want none", *metadata.Model)
				}
				return
			}
			if metadata.Model == nil || *metadata.Model != test.model {
				t.Fatalf("model = %v, want %s", metadata.Model, test.model)
			}
		})
	}
}

func TestMistralRejectsIncompleteOrUnsafePaths(t *testing.T) {
	t.Parallel()
	dialect := NewMistral()
	requests := []*ParsedRequest{
		{Method: http.MethodPost, Path: "/v1/ocr", Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{}`)},
		{Method: http.MethodPost, Path: "/v1/ocr", Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"model":"ocr","stream":true}`)},
		{Method: http.MethodGet, Path: "/mistral/v1/audio/voices/../secret"},
		{Method: http.MethodPost, Path: "/v1/chat/completions", Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"model":"m"}`)},
		{Method: http.MethodPost, Path: "/v1/audio/speech", Header: http.Header{"Content-Type": {"text/plain"}}, Body: []byte("hello")},
	}
	for _, request := range requests {
		if _, err := dialect.InspectRequest(request); err == nil {
			t.Fatalf("InspectRequest(%s %s) accepted invalid request", request.Method, request.Path)
		}
	}
}

func TestMistralOCRFileReferenceShape(t *testing.T) {
	for _, test := range []struct {
		name, document, want, invalidField string
	}{
		{"file", `{"type":"file","file_id":"file-synthetic"}`, "native/mistral/files/file-synthetic", ""},
		{"default-file-type", `{"file_id":"file-synthetic"}`, "native/mistral/files/file-synthetic", ""},
		{"nullable-default-type", `{"type":null,"file_id":"file-synthetic"}`, "native/mistral/files/file-synthetic", ""},
		{"document-url", `{"type":"document_url","document_url":"data:application/pdf;base64,synthetic"}`, "", ""},
		{"image-url", `{"type":"image_url","image_url":"data:image/png;base64,synthetic"}`, "", ""},
		{"missing-id", `{"type":"file"}`, "", "document.file_id"},
		{"null-id", `{"type":"file","file_id":null}`, "", "document.file_id"},
		{"empty-id", `{"type":"file","file_id":""}`, "", "document.file_id"},
		{"numeric-id", `{"type":"file","file_id":123}`, "", "document.file_id"},
		{"object-id", `{"type":"file","file_id":{"id":"file-synthetic"}}`, "", "document.file_id"},
		{"space-id", `{"type":"file","file_id":" file-synthetic "}`, "", "document.file_id"},
		{"path-id", `{"type":"file","file_id":"folder/file-synthetic"}`, "", "document.file_id"},
		{"encoded-id", `{"type":"file","file_id":"%66ile-synthetic"}`, "", "document.file_id"},
		{"conflicting-type", `{"type":"document_url","file_id":"file-synthetic","document_url":"https://synthetic.example/file.pdf"}`, "", "document.file_id"},
		{"numeric-type", `{"type":123,"file_id":"file-synthetic"}`, "", "document.type"},
		{"array-document", `[{"type":"file","file_id":"file-synthetic"}]`, "", "document"},
		{"null-document", `null`, "", "document"},
	} {
		t.Run(test.name, func(t *testing.T) {
			metadata, err := NewMistral().InspectRequest(&ParsedRequest{Method: http.MethodPost, Path: "/v1/ocr", Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"model":"mistral-ocr-4-1","document":` + test.document + `}`)})
			if test.invalidField != "" {
				var validation *RequestValidationError
				if !errors.As(err, &validation) || validation.Field != test.invalidField {
					t.Fatalf("expected fixed field validation for %s, got %v", test.invalidField, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				if len(metadata.ResourceReferences) != 0 {
					t.Fatal("URL input incorrectly requires a stored-file owner")
				}
			} else if len(metadata.ResourceReferences) != 1 || metadata.ResourceReferences[0] != test.want {
				t.Fatalf("references=%v", metadata.ResourceReferences)
			}
		})
	}
}

func TestMistralRewritesJSONAndMultipartModels(t *testing.T) {
	t.Parallel()
	dialect := NewMistral()
	rewritten, err := dialect.RewriteRequestModel(&ParsedRequest{
		Method: http.MethodPost, Path: "/v1/ocr",
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   []byte(`{"model":"public","document":{"type":"document_url"}}`),
	}, "upstream-ocr")
	if err != nil || !bytes.Contains(rewritten.Body, []byte(`"model":"upstream-ocr"`)) || !bytes.Contains(rewritten.Body, []byte(`"document"`)) {
		t.Fatalf("json rewrite = %s, err = %v", rewritten.Body, err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "public"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("language", "en"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	multipartRequest := &ParsedRequest{
		Method: http.MethodPost, Path: "/v1/audio/transcriptions",
		Header: http.Header{"Content-Type": {writer.FormDataContentType()}},
		Body:   body.Bytes(),
	}
	metadata, err := dialect.InspectRequest(multipartRequest)
	if err != nil || metadata.Model == nil || *metadata.Model != "public" || metadata.Operation != execution.OperationMistralAudioTranscription {
		t.Fatalf("multipart inspect = %+v, err = %v", metadata, err)
	}
	rewritten, err = dialect.RewriteRequestModel(multipartRequest, "upstream-audio")
	if err != nil || !bytes.Contains(rewritten.Body, []byte("upstream-audio")) || bytes.Contains(rewritten.Body, []byte("public")) {
		t.Fatalf("multipart rewrite = %q, err = %v", rewritten.Body, err)
	}
	if !strings.Contains(rewritten.Header.Get("Content-Type"), "multipart/form-data") {
		t.Fatalf("content type = %q", rewritten.Header.Get("Content-Type"))
	}

	response, err := dialect.RewriteResponseModel([]byte("not-json-audio"), "public")
	if err != nil || string(response) != "not-json-audio" {
		t.Fatalf("binary response rewrite = %q, err = %v", response, err)
	}
}

func TestInspectStandardRequestCoversMistralOCR(t *testing.T) {
	t.Parallel()
	metadata, err := InspectStandardRequest(protocol.Mistral, "mistral-ocr-latest")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Operation != execution.OperationMistralOCR || metadata.Model == nil || *metadata.Model != "mistral-ocr-latest" {
		t.Fatalf("metadata = %+v", metadata)
	}
}
