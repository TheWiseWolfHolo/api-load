package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"api-load/internal/models"

	"gorm.io/datatypes"
)

func TestMOD006AggregateGroupsSummarizeSubGroupModels(t *testing.T) {
	_, db, _, _ := newTestKeyService(t)
	if err := db.AutoMigrate(&models.GroupSubGroup{}); err != nil {
		t.Fatalf("auto migrate sub groups: %v", err)
	}

	aggregate := models.Group{
		Name:               "agg-model-summary",
		DisplayName:        "Aggregate Models",
		GroupType:          "aggregate",
		Upstreams:          []byte(`[]`),
		ChannelType:        "openai",
		TestModel:          "-",
		ValidationEndpoint: "",
	}
	if err := db.Create(&aggregate).Error; err != nil {
		t.Fatalf("create aggregate group: %v", err)
	}

	activeA := createServiceTestGroup(t, db)
	activeA.Name = "summary-a"
	activeA.DisplayName = "Summary A"
	if err := db.Save(&activeA).Error; err != nil {
		t.Fatalf("update subgroup a: %v", err)
	}
	activeB := createServiceTestGroup(t, db)
	activeB.Name = "summary-b"
	activeB.DisplayName = "Summary B"
	if err := db.Save(&activeB).Error; err != nil {
		t.Fatalf("update subgroup b: %v", err)
	}
	disabledByWeight := createServiceTestGroup(t, db)

	groupSvc := &GroupService{db: db}
	if err := groupSvc.SaveGroupModels(context.Background(), activeA.ID, []string{"gpt-shared", "gpt-a"}); err != nil {
		t.Fatalf("save models for subgroup a: %v", err)
	}
	if err := groupSvc.SaveGroupModels(context.Background(), activeB.ID, []string{"gpt-b", "gpt-shared"}); err != nil {
		t.Fatalf("save models for subgroup b: %v", err)
	}
	if err := groupSvc.SaveGroupModels(context.Background(), disabledByWeight.ID, []string{"gpt-disabled"}); err != nil {
		t.Fatalf("save models for disabled subgroup: %v", err)
	}

	if err := db.Create(&[]models.GroupSubGroup{
		{GroupID: aggregate.ID, SubGroupID: activeA.ID, Weight: 2},
		{GroupID: aggregate.ID, SubGroupID: activeB.ID, Weight: 1},
		{GroupID: aggregate.ID, SubGroupID: disabledByWeight.ID, Weight: 0},
	}).Error; err != nil {
		t.Fatalf("create subgroup links: %v", err)
	}

	svc := NewAggregateGroupService(db, nil)
	summary, err := svc.GetAggregateModelSummary(context.Background(), aggregate.ID)
	if err != nil {
		t.Fatalf("get aggregate model summary: %v", err)
	}

	gotIDs := make([]string, 0, len(summary.Models))
	for _, item := range summary.Models {
		gotIDs = append(gotIDs, item.ID)
	}
	if strings.Join(gotIDs, ",") != "gpt-a,gpt-b,gpt-shared" {
		t.Fatalf("expected deterministic deduplicated model union, got %#v", gotIDs)
	}

	shared := summary.Models[2]
	if shared.ID != "gpt-shared" || len(shared.Sources) != 2 {
		t.Fatalf("expected shared model to include two sources, got %#v", shared)
	}
	if shared.Sources[0].GroupID != activeA.ID || shared.Sources[0].Name != "summary-a" || shared.Sources[0].Weight != 2 {
		t.Fatalf("unexpected first shared source: %#v", shared.Sources[0])
	}
	if shared.Sources[1].GroupID != activeB.ID || shared.Sources[1].DisplayName != "Summary B" || shared.Sources[1].Weight != 1 {
		t.Fatalf("unexpected second shared source: %#v", shared.Sources[1])
	}
}

func TestMAP005AggregateModelListExposesAliasesAndManualOverrides(t *testing.T) {
	_, db, _, _ := newTestKeyService(t)
	if err := db.AutoMigrate(&models.GroupSubGroup{}); err != nil {
		t.Fatalf("auto migrate sub groups: %v", err)
	}

	active := createServiceTestGroup(t, db)
	disabledByWeight := createServiceTestGroup(t, db)
	aggregate := models.Group{
		Name:               "agg-external-models",
		GroupType:          "aggregate",
		Upstreams:          []byte(`[]`),
		ChannelType:        "openai",
		TestModel:          "-",
		ValidationEndpoint: "",
		Models:             datatypes.JSON([]byte(`["manual-alias"]`)),
	}
	mappings, err := json.Marshal([]ModelMappingRule{
		{
			Alias: "gpt-alias",
			Targets: []ModelMappingTarget{
				{SubGroupID: active.ID, Model: "gpt-real", Weight: 1},
				{SubGroupID: disabledByWeight.ID, Model: "gpt-disabled", Weight: 1},
			},
		},
		{
			Alias:   "disabled-alias",
			Targets: []ModelMappingTarget{{SubGroupID: disabledByWeight.ID, Model: "gpt-disabled", Weight: 1}},
		},
	})
	if err != nil {
		t.Fatalf("marshal mappings: %v", err)
	}
	aggregate.ModelMappings = datatypes.JSON(mappings)
	if err := db.Create(&aggregate).Error; err != nil {
		t.Fatalf("create aggregate group: %v", err)
	}
	if err := db.Create(&[]models.GroupSubGroup{
		{GroupID: aggregate.ID, SubGroupID: active.ID, Weight: 1},
		{GroupID: aggregate.ID, SubGroupID: disabledByWeight.ID, Weight: 0},
	}).Error; err != nil {
		t.Fatalf("create subgroup links: %v", err)
	}

	svc := NewAggregateGroupService(db, nil)
	models, err := svc.GetAggregateExternalModels(context.Background(), aggregate.ID)
	if err != nil {
		t.Fatalf("get aggregate external models: %v", err)
	}
	if strings.Join(models, ",") != "gpt-alias,manual-alias" {
		t.Fatalf("unexpected external models: %#v", models)
	}
}
