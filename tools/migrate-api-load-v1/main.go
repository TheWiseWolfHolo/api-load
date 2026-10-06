// migrate-api-load-v1 imports an archived API-Load snapshot into an empty v2 database.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gpt-load/internal/channel"
	"gpt-load/internal/platform/encryption"
	"gpt-load/internal/storage"
	"gpt-load/internal/storage/models"
)

type legacyCredential struct {
	ID             uint       `json:"id"`
	GroupID        uint       `json:"group_id"`
	PoolID         uint       `json:"resource_pool_id"`
	Value          string     `json:"key_value"`
	Notes          string     `json:"notes"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	Enabled        *bool      `json:"enabled"`
	Weight         int        `json:"weight"`
	Cooldown       *time.Time `json:"cooldown_until"`
	GlobalCooldown *time.Time `json:"global_cooldown_until"`
}

type legacyGroup struct {
	ID         uint              `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"group_type"`
	Channel    string            `json:"channel_type"`
	PoolID     *uint             `json:"resource_pool_id"`
	EndpointID *uint             `json:"resource_endpoint_id"`
	ProxyKeys  string            `json:"proxy_keys"`
	Models     []string          `json:"models"`
	TestModel  string            `json:"test_model"`
	Redirects  map[string]string `json:"model_redirect_rules"`
	Config     map[string]any    `json:"config"`
	Params     map[string]any    `json:"param_overrides"`
	Headers    []struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Action string `json:"action"`
	} `json:"header_rules"`
	Upstreams []struct {
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	} `json:"upstreams"`
}

type legacyEndpoint struct {
	ID      uint   `json:"id"`
	PoolID  uint   `json:"resource_pool_id"`
	URL     string `json:"base_url"`
	Channel string `json:"channel_type"`
	Enabled *bool  `json:"enabled"`
}

type snapshot struct {
	ExtraModels map[string][]string `json:"-"`
	Groups      []legacyGroup       `json:"groups"`
	Keys        []legacyCredential  `json:"api_keys"`
	Resources   []legacyCredential  `json:"upstream_resources"`
	Endpoints   []legacyEndpoint    `json:"resource_pool_endpoints"`
	Settings    []struct {
		Key   string `json:"setting_key"`
		Value string `json:"setting_value"`
	} `json:"system_settings"`
	Logs []struct {
		GroupID uint   `json:"group_id"`
		Model   string `json:"model"`
	} `json:"request_logs"`
}

type modelPair struct {
	ID    string `json:"id"`
	Alias string `json:"alias,omitempty"`
}
type groupReport struct {
	LegacyID     uint   `json:"legacy_id"`
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Credentials  int    `json:"credentials"`
	Disabled     int    `json:"disabled"`
	Deduplicated int    `json:"deduplicated"`
	AccessKey    string `json:"access_key"`
	Models       int    `json:"models"`
}
type migrationReport struct {
	Groups          []groupReport `json:"groups"`
	SourceGroups    int           `json:"source_groups"`
	SourceKeys      int           `json:"source_keys"`
	SourceResources int           `json:"source_resources"`
	Warnings        []string      `json:"warnings"`
}

func main() {
	input := flag.String("input", "", "protected legacy.json snapshot")
	deployment := flag.String("deployment", "", "protected deployment-before.json containing the old encryption configuration")
	keyFile := flag.String("target-key-file", "", "new v2 encryption.key")
	output := flag.String("output", "", "new protected report directory")
	extraModels := flag.String("models-file", "", "optional New API group-to-model mapping")
	flag.Parse()
	if *input == "" || *deployment == "" || *keyFile == "" || *output == "" || os.Getenv("MIGRATION_TARGET_DSN") == "" {
		fmt.Fprintln(os.Stderr, "input, deployment, target-key-file, output and MIGRATION_TARGET_DSN are required")
		os.Exit(2)
	}
	if err := run(*input, *deployment, *keyFile, *output, *extraModels, os.Getenv("MIGRATION_TARGET_DSN")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func oldKeyFromDeployment(raw []byte) (string, error) {
	var deployment struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Env []struct {
							Name  string `json:"name"`
							Value string `json:"value"`
						} `json:"env"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &deployment); err != nil {
		return "", errors.New("invalid deployment backup")
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, entry := range container.Env {
			if entry.Name == "ENCRYPTION_KEY" {
				return entry.Value, nil
			}
		}
	}
	return "", nil
}

func legacyDecrypt(key, value string) (string, error) {
	decrypt, err := newLegacyDecryptor(key)
	if err != nil {
		return "", err
	}
	return decrypt(value)
}

func newLegacyDecryptor(key string) (func(string) (string, error), error) {
	if key == "" {
		return func(value string) (string, error) { return value, nil }, nil
	}
	derived := pbkdf2.Key([]byte(key), []byte("gpt-load-encryption-v1"), 100000, 32, sha256.New)
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return func(value string) (string, error) {
		data, err := hex.DecodeString(value)
		if err != nil {
			return "", errors.New("legacy credential is not valid ciphertext")
		}
		if len(data) < gcm.NonceSize() {
			return "", errors.New("legacy credential ciphertext is too short")
		}
		plaintext, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
		if err != nil {
			return "", errors.New("legacy credential decryption failed; check the original encryption key")
		}
		return string(plaintext), nil
	}, nil
}

func run(input, deployment, keyFile, output, modelsFile, dsn string) error {
	raw, err := os.ReadFile(input)
	if err != nil {
		return errors.New("cannot read legacy snapshot")
	}
	var source snapshot
	if err := json.Unmarshal(raw, &source); err != nil {
		return errors.New("invalid legacy snapshot")
	}
	if len(source.Groups) == 0 {
		return errors.New("source has no groups")
	}
	if modelsFile != "" {
		rawModels, e := os.ReadFile(modelsFile)
		if e != nil {
			return errors.New("cannot read New API models")
		}
		if e := json.Unmarshal(rawModels, &source.ExtraModels); e != nil {
			return errors.New("invalid New API model mapping")
		}
	}
	config, err := os.ReadFile(deployment)
	if err != nil {
		return errors.New("cannot read deployment backup")
	}
	oldKey, err := oldKeyFromDeployment(config)
	if err != nil {
		return err
	}
	newKey, err := os.ReadFile(keyFile)
	if err != nil {
		return errors.New("cannot read target encryption key")
	}
	crypt, err := encryption.NewService(strings.TrimSpace(string(newKey)))
	if err != nil {
		return err
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		return errors.New("report directory must not already exist")
	}
	db, err := storage.Open(dsn)
	if err != nil {
		return errors.New("cannot open target database")
	}
	defer func() {
		sqlDB, e := db.DB()
		if e == nil {
			_ = sqlDB.Close()
		}
	}()
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return errors.New("cannot inspect target database")
	}
	if len(tables) != 0 {
		return errors.New("target database must be completely empty")
	}
	if err := storage.AutoMigrate(db); err != nil {
		return errors.New("v2 schema initialization failed")
	}
	report, err := importSnapshot(db, source, oldKey, crypt, time.Now())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		return err
	}
	archive, err := crypt.Encrypt(string(raw))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "legacy-archive.enc"), []byte(archive), 0600); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "routes.json"), encoded, 0600); err != nil {
		return err
	}
	credentials, disabled := 0, 0
	for _, group := range report.Groups {
		credentials += group.Credentials
		disabled += group.Disabled
	}
	fmt.Printf("Imported %d groups, %d credentials (%d disabled); source archive and private routes saved\n", len(report.Groups), credentials, disabled)
	return nil
}

func importSnapshot(db *gorm.DB, source snapshot, oldKey string, crypt encryption.Service, now time.Time) (migrationReport, error) {
	report := migrationReport{SourceGroups: len(source.Groups), SourceKeys: len(source.Keys), SourceResources: len(source.Resources), Groups: []groupReport{}, Warnings: []string{}}
	registry, err := channel.CompileRegistry()
	if err != nil {
		return report, err
	}
	decryptLegacy, err := newLegacyDecryptor(oldKey)
	if err != nil {
		return report, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Group{}).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errors.New("target contains groups; refusing to overwrite")
		}
		sort.Slice(source.Groups, func(i, j int) bool { return source.Groups[i].ID < source.Groups[j].ID })
		legacyGrant := map[string]map[uint]bool{}
		for _, old := range source.Groups {
			if old.Kind != "standard" && old.Kind != "" {
				return fmt.Errorf("unsupported legacy group type for group %d", old.ID)
			}
			if old.Channel != "openai" && old.Channel != "openai-response" && old.Channel != "anthropic" && old.Channel != "gemini" {
				return fmt.Errorf("unsupported channel for group %d", old.ID)
			}
			channelID := channel.ID(old.Channel)
			if old.Channel == "openai" {
				channelID = channel.ID("openai_compatible")
			}
			if old.Name == "mistral" {
				channelID = channel.ID("mistral")
			}
			if old.Name == "cohere-native" {
				channelID = channel.ID("cohere")
			}
			if old.Channel == "openai-response" {
				channelID = channel.ID("openai")
			}
			urls := []string{}
			weights := []int{}
			enabled := true
			if old.PoolID != nil {
				found := false
				for _, endpoint := range source.Endpoints {
					if old.EndpointID != nil && endpoint.ID == *old.EndpointID && endpoint.PoolID == *old.PoolID {
						urls = append(urls, endpoint.URL)
						weights = append(weights, 1)
						found = true
						enabled = endpoint.Enabled == nil || *endpoint.Enabled
					}
				}
				if !found {
					return fmt.Errorf("missing resource endpoint for group %d", old.ID)
				}
			} else {
				for _, upstream := range old.Upstreams {
					urls = append(urls, upstream.URL)
					weights = append(weights, upstream.Weight)
				}
			}
			if len(urls) == 0 {
				return fmt.Errorf("missing upstream for group %d", old.ID)
			}
			pairs := map[string]modelPair{}
			for _, name := range source.ExtraModels[old.Name] {
				if name != "" {
					pairs[name] = modelPair{ID: name}
				}
			}
			for _, name := range old.Models {
				if name != "" {
					pairs[name] = modelPair{ID: name}
				}
			}
			if old.TestModel != "" {
				pairs[old.TestModel] = modelPair{ID: old.TestModel}
			}
			for _, entry := range source.Logs {
				if entry.GroupID == old.ID && entry.Model != "" {
					pairs[entry.Model] = modelPair{ID: entry.Model}
				}
			}
			for alias, target := range old.Redirects {
				if target != "" {
					pairs[alias] = modelPair{ID: target, Alias: alias}
				}
			}
			modelList := []modelPair{}
			for _, pair := range pairs {
				modelList = append(modelList, pair)
			}
			sort.Slice(modelList, func(i, j int) bool {
				if modelList[i].ID == modelList[j].ID {
					return modelList[i].Alias < modelList[j].Alias
				}
				return modelList[i].ID < modelList[j].ID
			})
			modelJSON, _ := json.Marshal(modelList)
			overrides := map[string]any{}
			for _, field := range []string{"request_timeout", "blacklist_threshold"} {
				if value, ok := old.Config[field]; ok {
					overrides[field] = value
				}
			}
			if strategy, ok := old.Config["key_selection_strategy"].(string); ok {
				overrides["affinity_enabled"] = strategy == "sticky" || strategy == "fill_first"
			}
			if len(old.Headers) > 0 {
				set := map[string]string{}
				remove := []string{}
				for _, h := range old.Headers {
					if h.Action == "remove" {
						remove = append(remove, h.Key)
					} else {
						set[h.Key] = h.Value
					}
				}
				overrides["header_rules"] = map[string]any{"set": set, "remove": remove}
			}
			if len(old.Params) > 0 {
				overrides["parameter_overrides"] = []any{map[string]any{"match": map[string]any{}, "set": old.Params}}
			}
			overrideJSON, _ := json.Marshal(overrides)
			for index, baseURL := range urls {
				baseURL = migratedBaseURL(string(channelID), baseURL)
				paramsJSON, _ := json.Marshal(map[string]string{"base_url": baseURL})
				params, e := registry.ValidateParams(channelID, paramsJSON)
				if e != nil {
					return fmt.Errorf("invalid upstream address for group %d", old.ID)
				}
				name := old.Name
				if index > 0 {
					name = fmt.Sprintf("%s-upstream-%d", old.Name, index+1)
				}
				weight := weights[index]
				if weight < 1 {
					weight = 1
				}
				group := models.Group{Name: name, ChannelID: string(channelID), ConnectionType: models.ConnectionTypeAPIKey, Params: models.JSON(params.CanonicalJSON()), Models: modelJSON, Overrides: overrideJSON, Enabled: enabled, WeightManual: &weight, CreatedAtMS: now.UnixMilli(), UpdatedAtMS: now.UnixMilli()}
				if e := tx.Create(&group).Error; e != nil {
					return fmt.Errorf("create group %d failed", old.ID)
				}
				gr := groupReport{LegacyID: old.ID, ID: group.ID, Name: name, Models: len(modelList)}
				pending := []struct {
					row     legacyCredential
					dormant bool
				}{}
				if old.PoolID != nil {
					for _, key := range source.Resources {
						if key.PoolID == *old.PoolID {
							pending = append(pending, struct {
								row     legacyCredential
								dormant bool
							}{key, false})
						}
					}
				}
				for _, key := range source.Keys {
					if key.GroupID == old.ID {
						pending = append(pending, struct {
							row     legacyCredential
							dormant bool
						}{key, old.PoolID != nil})
					}
				}
				seen := map[string]bool{}
				for _, item := range pending {
					key, e := decryptLegacy(item.row.Value)
					if e != nil {
						return fmt.Errorf("decrypt credential %d in group %d failed", item.row.ID, old.ID)
					}
					if strings.TrimSpace(key) == "" {
						return fmt.Errorf("empty credential %d in group %d", item.row.ID, old.ID)
					}
					credentialJSON, _ := json.Marshal(map[string]string{"api_key": key})
					canonical, e := registry.ValidateCredential(channelID, credentialJSON)
					if e != nil {
						return fmt.Errorf("invalid credential shape in group %d", old.ID)
					}
					plaintext := string(canonical.CanonicalJSON())
					fingerprint := crypt.Hash(plaintext)
					if seen[fingerprint] {
						gr.Deduplicated++
						continue
					}
					seen[fingerprint] = true
					ciphertext, e := crypt.Encrypt(plaintext)
					if e != nil {
						return e
					}
					status := models.CredentialStatusActive
					if item.dormant || (item.row.Enabled != nil && !*item.row.Enabled) || item.row.Status != "active" || (item.row.Cooldown != nil && item.row.Cooldown.After(now)) || (item.row.GlobalCooldown != nil && item.row.GlobalCooldown.After(now)) {
						status = models.CredentialStatusDisabled
						gr.Disabled++
					}
					label := item.row.Name
					if label == "" {
						label = item.row.Notes
					}
					if label == "" {
						label = fmt.Sprintf("legacy-%d", item.row.ID)
					}
					if item.row.Status != "active" {
						label = label + " [legacy " + item.row.Status + "]"
					}
					if item.dormant {
						label = label + " [legacy dormant]"
					}
					credentialWeight := item.row.Weight
					if credentialWeight < 1 {
						credentialWeight = 1
					}
					credential := models.Credential{GroupID: group.ID, Name: label, Data: ciphertext, Fingerprint: fingerprint, IdentityFingerprint: fingerprint, Status: status, WeightManual: &credentialWeight, CreatedAtMS: now.UnixMilli(), UpdatedAtMS: now.UnixMilli()}
					if e := tx.Create(&credential).Error; e != nil {
						return fmt.Errorf("insert credential in group %d failed", old.ID)
					}
					gr.Credentials++
				}
				var random [32]byte
				if _, e := rand.Read(random[:]); e != nil {
					return e
				}
				gr.AccessKey = "sk-al-" + hex.EncodeToString(random[:])
				if e := insertAccessKey(tx, crypt, "New API / "+name, gr.AccessKey, []uint{group.ID}); e != nil {
					return e
				}
				for _, key := range strings.Split(old.ProxyKeys, ",") {
					key = strings.TrimSpace(key)
					if key != "" {
						if legacyGrant[key] == nil {
							legacyGrant[key] = map[uint]bool{}
						}
						legacyGrant[key][group.ID] = true
					}
				}
				report.Groups = append(report.Groups, gr)
			}
		}
		for _, setting := range source.Settings {
			if setting.Key == "proxy_keys" {
				for _, key := range strings.Split(setting.Value, ",") {
					key = strings.TrimSpace(key)
					if key != "" {
						legacyGrant[key] = map[uint]bool{}
						for _, group := range report.Groups {
							legacyGrant[key][group.ID] = true
						}
					}
				}
			}
		}
		for key, grant := range legacyGrant {
			ids := []uint{}
			for id := range grant {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			if e := insertAccessKey(tx, crypt, "Migrated legacy access", key, ids); e != nil {
				return e
			}
		}
		for _, setting := range source.Settings {
			name := setting.Key
			if name == "max_retries" {
				name = "retry_count"
			}
			if name != "request_timeout" && name != "retry_count" && name != "blacklist_threshold" && name != "request_log_retention_days" {
				continue
			}
			if name == "request_log_retention_days" && setting.Value == "0" {
				continue
			}
			var value any
			if e := json.Unmarshal([]byte(setting.Value), &value); e != nil {
				return fmt.Errorf("invalid legacy setting %s", name)
			}
			if e := tx.Create(&models.SystemSetting{Key: name, Value: setting.Value, UpdatedAtMS: now.UnixMilli()}).Error; e != nil {
				return fmt.Errorf("migrate setting %s failed", name)
			}
		}
		return nil
	})
	return report, err
}

func insertAccessKey(tx *gorm.DB, crypt encryption.Service, name, key string, ids []uint) error {
	encrypted, err := crypt.Encrypt(key)
	if err != nil {
		return err
	}
	filters, _ := json.Marshal(map[string]any{"groups": ids, "protocols": []string{}, "models": []string{}})
	prefix := ""
	suffix := "****"
	if len(key) > 8 {
		suffix = key[len(key)-4:]
	}
	if len(key) > 16 {
		prefix = key[:6]
	}
	row := models.AccessKey{Name: name, KeyValue: encrypted, KeyHash: crypt.Hash(key), KeyPrefix: &prefix, KeySuffix: suffix, Status: "active", Filters: filters}
	if err := tx.Create(&row).Error; err != nil {
		return errors.New("create access credential failed")
	}
	return nil
}

func migratedBaseURL(channelID, baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	version := ""
	switch channelID {
	case "openai_compatible", "mistral":
		version = "/v1"
	case "cohere":
		version = "/v2"
	case "gemini":
		version = "/v1beta"
	}
	if version != "" && !strings.HasSuffix(baseURL, version) {
		baseURL += version
	}
	return baseURL
}
