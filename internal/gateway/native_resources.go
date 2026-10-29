package gateway

import (
	"context"
	"encoding/json"
	"errors"
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
	Claim(context.Context, state.ResponseBinding) (state.ResponseBinding, error)
	Load(context.Context, uint, string) (state.ResponseBinding, bool, error)
}

// SetResourceBindingStore 在依赖装配阶段、HTTP 流量到达前接入持久层；未接入时只用内存归属。
func (handler *Handler) SetResourceBindingStore(store ResourceBindingStore) {
	handler.resourceBindingStore = store
}

// recordResourceBinding 先确认持久归属，再缓存数据库返回的原到期时间。
// 数据库暂不可用时保留当前创建归属；确定的归属冲突不能使用内存降级。
func (handler *Handler) recordResourceBinding(accessKeyID uint, key string, ref state.CredentialRef) bool {
	binding, valid := handler.responseBindings.PrepareResource(accessKeyID, key, ref)
	if !valid {
		return false
	}
	if handler.resourceBindingStore == nil {
		return handler.responseBindings.Remember(binding)
	}
	ctx, cancel := context.WithTimeout(context.Background(), resourceBindingStoreTimeout)
	defer cancel()
	canonical, err := handler.resourceBindingStore.Claim(ctx, binding)
	if errors.Is(err, state.ErrResourceBindingConflict) {
		return false
	}
	if err != nil {
		handler.logger.WithFields(logrus.Fields{
			"event":         "resource_binding.persist_failed",
			"access_key_id": accessKeyID,
			"group_id":      ref.GroupID,
			"credential_id": ref.ID,
			"error_class":   "store_unavailable",
		}).Warn("resource ownership is kept in memory only")
	} else {
		binding = canonical
	}
	return handler.responseBindings.Remember(binding)
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
	if !handler.responseBindings.Remember(binding) {
		return state.ResponseBinding{}, false, errors.New("resource ownership could not be accepted by the cache")
	}
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
