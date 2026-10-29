package models

// ResourceBinding 保存原生资源（文件、批任务等）属于哪个访问令牌和哪把凭据，不包含密钥或资源内容。
// ResourceHash 是 ResourceKey 的 SHA-256 十六进制值，用来建唯一索引，避免长 ID 超出索引长度。
type ResourceBinding struct {
	ID                 uint   `gorm:"primaryKey;autoIncrement"`
	AccessKeyID        uint   `gorm:"not null;check:chk_resource_bindings_access_key,access_key_id > 0;uniqueIndex:idx_resource_bindings_identity,priority:1"`
	ResourceHash       string `gorm:"type:char(64);not null;uniqueIndex:idx_resource_bindings_identity,priority:2"`
	ResourceKey        string `gorm:"type:text;not null"`
	GroupID            uint   `gorm:"not null;check:chk_resource_bindings_group,group_id > 0"`
	CredentialID       uint   `gorm:"not null;check:chk_resource_bindings_credential,credential_id > 0"`
	IdentityGeneration uint64 `gorm:"not null;check:chk_resource_bindings_generation,identity_generation > 0"`
	ExpiresAtMS        int64  `gorm:"column:expires_at_ms;not null;check:chk_resource_bindings_expiry,expires_at_ms > 0;index:idx_resource_bindings_expiry"`
}

func (ResourceBinding) TableName() string { return "resource_bindings" }
