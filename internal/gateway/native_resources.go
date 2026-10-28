package gateway

import (
	"encoding/json"
	"fmt"

	"gpt-load/internal/dialect"
	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
)

func (handler *Handler) nativeResourceObserver(accessKeyID uint, selection scheduler.Selection, ref state.CredentialRef, request *dialect.ParsedRequest) func([]byte) error {
	e, err := dialect.ClassifyNativeService(request.Method, request.Path)
	if err != nil {
		return nil
	}
	return func(payload []byte) error {
		var body struct {
			ID         string `json:"id"`
			RequestID  string `json:"requestId"`
			OutputFile string `json:"output_file"`
			ErrorFile  string `json:"error_file"`
		}
		if json.Unmarshal(payload, &body) != nil {
			if e.CreatesResource {
				return fmt.Errorf("%w: invalid resource creation response", ErrUpstreamProtocol)
			}
			return nil
		}
		if e.Collection == "video" {
			body.ID = body.RequestID
		}
		if e.CreatesResource && body.ID == "" {
			return fmt.Errorf("%w: resource ID missing", ErrUpstreamProtocol)
		}
		if e.CreatesResource && !handler.responseBindings.Record(accessKeyID, dialect.NativeResourceKey(e.Service, e.Collection, body.ID), ref) {
			return fmt.Errorf("%w: resource ownership could not be recorded", ErrUpstreamProtocol)
		}
		if e.Collection == "batch/jobs" {
			for _, id := range []string{body.OutputFile, body.ErrorFile} {
				if id != "" && !handler.responseBindings.Record(accessKeyID, dialect.NativeResourceKey(e.Service, "files", id), ref) {
					return fmt.Errorf("%w: output file ownership could not be recorded", ErrUpstreamProtocol)
				}
			}
		}
		return nil
	}
}
