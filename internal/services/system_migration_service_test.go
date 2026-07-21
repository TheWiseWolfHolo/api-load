package services

import (
	"context"
	"encoding/json"
	"testing"

	"api-load/internal/models"

	"gorm.io/datatypes"
)

func TestMIG001FullSystemExportContainsVersionedEnvelope(t *testing.T) {
	keySvc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, keySvc, group.ID, "sk-test-migration", "notes", models.KeyStatusActive, 0, 0)

	exportSvc := NewSystemExportService(db, keySvc.EncryptionSvc)
	envelope, err := exportSvc.Export(context.Background(), SystemExportOptions{Mode: ExportModePlain, ConfirmPlain: true})
	if err != nil {
		t.Fatalf("export system: %v", err)
	}
	if envelope.Version == "" || envelope.ExportedAt.IsZero() || len(envelope.Groups) != 1 || len(envelope.Groups[0].Keys) != 1 {
		t.Fatalf("unexpected migration envelope: %#v", envelope)
	}
}

func TestMIG002ExportModesEnforceKeySafety(t *testing.T) {
	keySvc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, keySvc, group.ID, "sk-test-sensitive", "notes", models.KeyStatusActive, 0, 0)
	exportSvc := NewSystemExportService(db, keySvc.EncryptionSvc)

	if _, err := exportSvc.Export(context.Background(), SystemExportOptions{Mode: ExportModePlain}); err == nil {
		t.Fatal("expected plain export to require confirmation")
	}

	masked, err := exportSvc.Export(context.Background(), SystemExportOptions{Mode: ExportModeMasked})
	if err != nil {
		t.Fatalf("masked export: %v", err)
	}
	if masked.Groups[0].Keys[0].Key != "" || masked.Groups[0].Keys[0].MaskedKey == "sk-test-sensitive" {
		t.Fatalf("masked export exposed real key: %#v", masked.Groups[0].Keys[0])
	}

	configOnly, err := exportSvc.Export(context.Background(), SystemExportOptions{Mode: ExportModeConfigOnly})
	if err != nil {
		t.Fatalf("config-only export: %v", err)
	}
	if len(configOnly.Groups[0].Keys) != 0 {
		t.Fatalf("config-only export included key material: %#v", configOnly.Groups[0].Keys)
	}
}

func TestMIG003FullSystemImportPreviewReportsChangesWithoutMutation(t *testing.T) {
	keySvc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, keySvc, group.ID, "sk-test-existing", "old", models.KeyStatusActive, 0, 0)

	importSvc := NewSystemImportService(db, keySvc, keySvc.EncryptionSvc)
	preview, err := importSvc.Preview(context.Background(), SystemExportEnvelope{Groups: []SystemExportGroup{
		{Name: group.Name, ChannelType: group.ChannelType, Keys: []SystemExportKey{
			{Key: "sk-test-existing", Notes: "new", Status: models.KeyStatusDisabled},
			{Key: "sk-test-new", Notes: "new", Status: models.KeyStatusActive},
		}},
	}})
	if err != nil {
		t.Fatalf("preview import: %v", err)
	}
	if preview.DuplicateKeys != 1 || preview.NewKeys != 1 || preview.NotesUpdates != 1 || preview.OverwrittenGroups != 1 {
		t.Fatalf("unexpected preview: %#v", preview)
	}

	var count int64
	if err := db.Model(&models.APIKey{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if count != 1 {
		t.Fatalf("preview mutated keys, count=%d", count)
	}
}

func TestMIG004FullSystemImportExportRoundTripPreservesSupportedFields(t *testing.T) {
	sourceKeySvc, sourceDB, _, encryptionSvc := newTestKeyService(t)
	sourceGroup := createServiceTestGroup(t, sourceDB)
	sourceGroup.Description = "migration description"
	sourceGroup.Sort = 9
	sourceGroup.TestModel = "gpt-test-model"
	sourceGroup.ValidationEndpoint = "/v1/test"
	sourceGroup.Upstreams = datatypes.JSON([]byte(`[{"url":"https://upstream.example.invalid","weight":7}]`))
	sourceGroup.Config = datatypes.JSONMap{
		"key_selection_strategy": "fill_first",
		"fill_cooldown_minutes":  3,
		"proxy_url":              "http://user:pass@example.invalid:8080",
	}
	sourceGroup.HeaderRules = datatypes.JSON([]byte(`[{"key":"X-Test","value":"ok","action":"set"}]`))
	sourceGroup.ParamOverrides = datatypes.JSONMap{"temperature": 0.7}
	sourceGroup.ModelRedirectRules = datatypes.JSONMap{"gpt-alias": "gpt-real"}
	sourceGroup.ModelRedirectStrict = true
	sourceGroup.Models = datatypes.JSON([]byte(`["gpt-alias","gpt-real"]`))
	sourceGroup.ModelMappings = datatypes.JSON([]byte(`[{"alias":"gpt-alias","targets":[{"sub_group_id":1,"model":"gpt-real","weight":1}]}]`))
	if err := sourceDB.Save(&sourceGroup).Error; err != nil {
		t.Fatalf("save source group fields: %v", err)
	}
	seedKey(t, sourceKeySvc, sourceGroup.ID, "sk-test-roundtrip", "paused", models.KeyStatusDisabled, 3, 42)

	exportSvc := NewSystemExportService(sourceDB, encryptionSvc)
	envelope, err := exportSvc.Export(context.Background(), SystemExportOptions{Mode: ExportModePlain, ConfirmPlain: true})
	if err != nil {
		t.Fatalf("export source: %v", err)
	}

	targetKeySvc, targetDB, _, targetEncryption := newTestKeyService(t)
	importSvc := NewSystemImportService(targetDB, targetKeySvc, targetEncryption)
	if err := importSvc.Import(context.Background(), envelope); err != nil {
		t.Fatalf("import target: %v", err)
	}

	var imported models.APIKey
	if err := targetDB.Where("key_hash = ?", targetEncryption.Hash("sk-test-roundtrip")).First(&imported).Error; err != nil {
		t.Fatalf("load imported key: %v", err)
	}
	if imported.Notes != "paused" || imported.Status != models.KeyStatusActive || models.CredentialEnabled(imported.Enabled) || imported.RequestCount != 42 {
		t.Fatalf("round trip did not preserve supported key fields: %#v", imported)
	}

	var importedGroup models.Group
	if err := targetDB.Where("name = ?", sourceGroup.Name).First(&importedGroup).Error; err != nil {
		t.Fatalf("load imported group: %v", err)
	}
	if importedGroup.Description != sourceGroup.Description ||
		importedGroup.Sort != sourceGroup.Sort ||
		importedGroup.TestModel != sourceGroup.TestModel ||
		importedGroup.ValidationEndpoint != sourceGroup.ValidationEndpoint ||
		!importedGroup.ModelRedirectStrict {
		t.Fatalf("round trip did not preserve scalar group fields: %#v", importedGroup)
	}
	assertJSONEqual(t, "upstreams", sourceGroup.Upstreams, importedGroup.Upstreams)
	assertJSONEqual(t, "header rules", sourceGroup.HeaderRules, importedGroup.HeaderRules)
	assertJSONEqual(t, "models", sourceGroup.Models, importedGroup.Models)
	assertJSONEqual(t, "model mappings", sourceGroup.ModelMappings, importedGroup.ModelMappings)
	if importedGroup.Config["key_selection_strategy"] != "fill_first" ||
		importedGroup.Config["proxy_url"] != "http://user:pass@example.invalid:8080" ||
		importedGroup.ParamOverrides["temperature"] == nil ||
		importedGroup.ModelRedirectRules["gpt-alias"] != "gpt-real" {
		t.Fatalf("round trip did not preserve config maps: %#v", importedGroup)
	}
}

func assertJSONEqual(t *testing.T, label string, want []byte, got []byte) {
	t.Helper()
	var wantValue any
	var gotValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unmarshal wanted %s: %v", label, err)
	}
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got %s: %v", label, err)
	}
	if jsonString(t, wantValue) != jsonString(t, gotValue) {
		t.Fatalf("%s mismatch want=%s got=%s", label, jsonString(t, wantValue), jsonString(t, gotValue))
	}
}

func jsonString(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json value: %v", err)
	}
	return string(data)
}
