package dialect

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
)

func decisionsPayload(request *ParsedRequest) ([]byte, string, error) {
	value := request.Header.Get("Content-Type")
	if value == "" {
		return request.Body, "", nil
	}
	media, params, err := mime.ParseMediaType(value)
	if err != nil {
		return nil, "", &RequestValidationError{Field: "content_type", Rule: "invalid_content_type"}
	}
	if media == "application/json" {
		return request.Body, "", nil
	}
	if media != "multipart/form-data" || params["boundary"] == "" {
		return nil, "", &RequestValidationError{Field: "content_type", Rule: "json_or_multipart_required"}
	}
	reader := multipart.NewReader(bytes.NewReader(request.Body), params["boundary"])
	var body []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("invalid decisions multipart")
		}
		if part.FormName() == "request" {
			if body != nil {
				return nil, "", &RequestValidationError{Field: "request", Rule: "duplicate_field"}
			}
			body, err = io.ReadAll(part)
		} else {
			_, err = io.Copy(io.Discard, part)
		}
		if err != nil {
			return nil, "", fmt.Errorf("read decisions multipart")
		}
	}
	if len(body) == 0 {
		return nil, "", &RequestValidationError{Field: "request", Rule: "required_json_part"}
	}
	return body, params["boundary"], nil
}

func rewriteDecisionsMultipart(original, body []byte, boundary string) ([]byte, error) {
	reader := multipart.NewReader(bytes.NewReader(original), boundary)
	var result bytes.Buffer
	writer := multipart.NewWriter(&result)
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, err
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		out, err := writer.CreatePart(part.Header)
		if err != nil {
			return nil, err
		}
		if part.FormName() == "request" {
			_, err = out.Write(body)
		} else {
			_, err = io.Copy(out, part)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}
