package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"gpt-load/internal/dialect"
	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
)

// resourceBindingStoreTimeout 限制单次归属读写占用请求的时间。
const resourceBindingStoreTimeout = 3 * time.Second

// ResourceBindingStore 持久保存原生资源归属；内存中的 responseBindings 仍是首选查询来源。
type ResourceBindingStore interface {
	Save(context.Context, state.ResponseBinding) error
	Load(context.Context, uint, string) (state.ResponseBinding, bool, error)
}

// SetResourceBindingStore 在依赖装配阶段、HTTP 流量到达前接入持久层；未接入时只用内存归属。
func (handler *Handler) SetResourceBindingStore(store ResourceBindingStore) {
	handler.resourceBindingStore = store
}

// recordResourceBinding 先在内存登记，冲突时拒绝；持久化失败只记日志，当前进程内的归属仍然有效。
func (handler *Handler) recordResourceBinding(accessKeyID uint, key string, ref state.CredentialRef) bool {
	if !handler.responseBindings.Record(accessKeyID, key, ref) {
		return false
	}
	if handler.resourceBindingStore == nil {
		return true
	}
	binding, found := handler.responseBindings.Lookup(accessKeyID, key)
	if !found {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), resourceBindingStoreTimeout)
	defer cancel()
	if err := handler.resourceBindingStore.Save(ctx, binding); err != nil {
		handler.logger.WithFields(logrus.Fields{
			"event":         "resource_binding.persist_failed",
			"access_key_id": accessKeyID,
			"group_id":      ref.GroupID,
			"credential_id": ref.ID,
		}).WithError(err).Warn("resource ownership is kept in memory only")
	}
	return true
}

// lookupResourceBinding 在内存未命中时回查持久层，并把找到的归属放回内存。
func (handler *Handler) lookupResourceBinding(ctx context.Context, accessKeyID uint, key string) (state.ResponseBinding, bool, error) {
	if binding, found := handler.responseBindings.Lookup(accessKeyID, key); found {
		return binding, true, nil
	}
	if handler.resourceBindingStore == nil || !state.IsResourceBindingKey(key) {
		return state.ResponseBinding{}, false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, resourceBindingStoreTimeout)
	defer cancel()
	binding, found, err := handler.resourceBindingStore.Load(ctx, accessKeyID, key)
	if err != nil || !found {
		return state.ResponseBinding{}, false, err
	}
	handler.responseBindings.Remember(binding)
	return binding, true, nil
}

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
		if e.CreatesResource && !handler.recordResourceBinding(accessKeyID, dialect.NativeResourceKey(e.Service, e.Collection, body.ID), ref) {
			return fmt.Errorf("%w: resource ownership could not be recorded", ErrUpstreamProtocol)
		}
		if e.Collection == "batch/jobs" {
			for _, id := range []string{body.OutputFile, body.ErrorFile} {
				if id != "" && !handler.recordResourceBinding(accessKeyID, dialect.NativeResourceKey(e.Service, "files", id), ref) {
					return fmt.Errorf("%w: output file ownership could not be recorded", ErrUpstreamProtocol)
				}
			}
		}
		return nil
	}
}
