package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func newAdminArkAssetTestService(t *testing.T, handler http.HandlerFunc) (*Service, *gorm.DB, *model.User) {
	t.Helper()
	t.Setenv("CANVAS_ALLOW_PRIVATE_UPSTREAMS", "true")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.ArkPrivateAssetBinding{}, &model.ArkPortraitAsset{}, &model.Resource{}, &model.Task{}, &model.CanvasProject{}, &model.Asset{}, &model.AssetVersion{}, &model.Project{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc := New(repository.New(db), t.TempDir())
	if err := svc.saveArkPrivateAssetSetting(&model.SystemSetting{Key: arkPrivateAssetSettingKey}, arkPrivateAssetSettingValue{Region: "cn-beijing", ProjectName: "project-test", AccessKeyID: "test-ak", AccessKeySecret: "test-sk"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	previous := arkPrivateAssetAPIBaseURLOverride
	arkPrivateAssetAPIBaseURLOverride = server.URL
	t.Cleanup(func() { arkPrivateAssetAPIBaseURLOverride = previous })
	return svc, db, &model.User{ID: "admin", Role: model.UserRoleAdmin}
}

func TestAdminArkAssetsListAndRenameWhenAutomaticImportDisabled(t *testing.T) {
	var actions []string
	svc, _, admin := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("Action")
		actions = append(actions, action)
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["ProjectName"] != "project-test" {
			t.Errorf("wrong project: %v", body["ProjectName"])
		}
		switch action {
		case "ListAssets":
			if body["NextToken"] != "cursor" || body["PageNumber"] != nil {
				t.Errorf("wrong pagination: %#v", body)
			}
			filter := body["Filter"].(map[string]interface{})
			if filter["GroupType"] != "AIGC" || filter["Name"] != "hero" {
				t.Errorf("wrong filter: %#v", filter)
			}
			_, _ = w.Write([]byte(`{"Result":{"Items":[{"Id":"asset-test","Name":"hero","Status":"Active","URL":"https://example.com/a.png"}],"NextToken":"next"}}`))
		case "GetAsset":
			_, _ = w.Write([]byte(`{"Result":{"Id":"asset-test","ProjectName":"project-test","Status":"Active"}}`))
		case "UpdateAsset":
			if body["Name"] != "new name" || body["Id"] != "asset-test" {
				t.Errorf("wrong rename: %#v", body)
			}
			_, _ = w.Write([]byte(`{"Result":{"Id":"asset-test"}}`))
		default:
			t.Errorf("unexpected action %s", action)
		}
	})
	page, err := svc.AdminArkAssets(context.Background(), admin, AdminArkAssetQuery{Name: "hero", NextToken: "cursor"})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != "asset-test" || page.NextToken != "next" {
		t.Fatalf("list = %+v, %v", page, err)
	}
	if err := svc.RenameAdminArkAsset(context.Background(), admin, "asset-test", AdminArkAssetRenameRequest{Name: " new name "}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdminArkAssets(context.Background(), &model.User{ID: "ordinary"}, AdminArkAssetQuery{}); err == nil {
		t.Fatal("ordinary user must be rejected")
	}
	if err := svc.DeleteAdminArkAsset(context.Background(), nil, "asset-test"); err == nil {
		t.Fatal("anonymous delete must be rejected")
	}
	if err := svc.DeleteAdminArkAsset(context.Background(), &model.User{ID: "ordinary"}, "asset-test"); err == nil {
		t.Fatal("ordinary delete must be rejected")
	}
	if err := svc.RenameAdminArkAsset(context.Background(), &model.User{ID: "ordinary"}, "asset-test", AdminArkAssetRenameRequest{Name: "name"}); err == nil {
		t.Fatal("ordinary rename must be rejected")
	}
	if len(actions) != 3 {
		t.Fatalf("unexpected upstream calls: %v", actions)
	}
}

func TestManualArkUploadIgnoresAutomaticImportSwitch(t *testing.T) {
	svc, db, actor := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) { t.Error("approved binding should be reused") })
	resource := &model.Resource{ID: "resource-test", UserID: actor.ID, Status: model.ResourceStatusReady, MimeType: "image/png"}
	if err := db.Create(resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Resource{ID: "resource-unbound", UserID: actor.ID, Status: model.ResourceStatusReady, MimeType: "image/png"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ArkPrivateAssetBinding{ID: "binding", UserID: actor.ID, ResourceID: resource.ID, ProjectName: "project-test", ArkAssetID: "asset-test", Status: arkPrivateAssetStatusLive}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := svc.SyncResourceToArkPrivateAsset(context.Background(), actor, resource.ID)
	if err != nil || result.Status != "active" {
		t.Fatalf("manual sync = %+v, %v", result, err)
	}
	for _, taskUpload := range []string{"true", "false"} {
		input := canvasGenerationInput{Mode: "video", Config: providerConfig{InterfaceType: "volcengine-ark-video", ArkPrivateAssetUpload: taskUpload}, ReferenceImages: []providerMedia{{URL: "https://example.com/original.png", StorageKey: "resource:" + resource.ID}, {URL: "https://example.com/unbound.png", StorageKey: "resource:resource-unbound"}}}
		if err := svc.prepareArkPrivateAssetReferences(context.Background(), actor.ID, &input); err != nil {
			t.Fatal(err)
		}
		if input.ReferenceImages[0].URL != "asset://asset-test" {
			t.Fatal("manual upload was not reused with automatic import disabled")
		}
		if input.ReferenceImages[1].URL != "https://example.com/unbound.png" {
			t.Fatal("automatic import disabled but unbound image was uploaded")
		}
	}
}

func TestAdminArkAssetDeletionChecksUsageAndKeepsBindingOnFailure(t *testing.T) {
	deletes := 0
	failDelete := true
	svc, db, admin := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Action") == "GetAsset" {
			_, _ = w.Write([]byte(`{"Result":{"Id":"asset-test","ProjectName":"project-test"}}`))
			return
		}
		deletes++
		if failDelete {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"ResponseMetadata":{"Error":{"Code":"InternalError","Message":"unavailable"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"Result":{}}`))
	})
	binding := &model.ArkPrivateAssetBinding{ID: "binding", UserID: admin.ID, ResourceID: "resource-test", ProjectName: "project-test", ArkAssetID: "asset-test", Status: "active"}
	if err := db.Create(&model.Resource{ID: "resource-test", UserID: admin.ID, Provider: "local", Status: model.ResourceStatusReady, MimeType: "image/png"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatal(err)
	}
	canvas := &model.CanvasProject{ID: "canvas", Title: "引用画布", PayloadJSON: `{"arkAssetId":"asset-test"}`}
	if err := db.Create(canvas).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteAdminArkAsset(context.Background(), admin, "asset-test"); err == nil || !strings.Contains(err.Error(), "引用画布") {
		t.Fatalf("reference check = %v", err)
	}
	if deletes != 0 {
		t.Fatal("referenced asset was deleted")
	}
	if err := db.Delete(canvas).Error; err != nil {
		t.Fatal(err)
	}
	task := &model.Task{ID: "running-task", Status: model.TaskStatusRunning, InputJSON: `{"storageKey":"resource:resource-test"}`}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteAdminArkAsset(context.Background(), admin, "asset-test"); err == nil || !strings.Contains(err.Error(), "running-task") {
		t.Fatalf("active task check = %v", err)
	}
	if deletes != 0 {
		t.Fatal("asset in active task was deleted")
	}
	if err := db.Delete(task).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteAdminArkAsset(context.Background(), admin, "asset-test"); err == nil {
		t.Fatal("upstream failure was swallowed")
	}
	stored, err := svc.repo.ArkPrivateAssetBinding("resource-test", "project-test")
	if err != nil || stored.Status != "active" {
		t.Fatalf("binding after failure = %+v, %v", stored, err)
	}
	failDelete = false
	if err := svc.DeleteAdminArkAsset(context.Background(), admin, "asset-test"); err != nil {
		t.Fatal(err)
	}
	stored, err = svc.repo.ArkPrivateAssetBinding("resource-test", "project-test")
	if err != nil || stored.Status != "deleted" {
		t.Fatalf("binding after deletion = %+v, %v", stored, err)
	}
	if _, err := svc.Resource(admin.ID, "resource-test"); err != nil {
		t.Fatalf("original image was removed: %v", err)
	}
	_, setting, err := svc.readArkPrivateAssetSetting()
	if err != nil {
		t.Fatal(err)
	}
	setting.Enabled = true
	if err := svc.saveArkPrivateAssetSetting(&model.SystemSetting{Key: arkPrivateAssetSettingKey}, setting); err != nil {
		t.Fatal(err)
	}
	input := canvasGenerationInput{Mode: "video", Config: providerConfig{InterfaceType: "volcengine-ark-video"}, ReferenceImages: []providerMedia{{URL: "https://example.com/original.png", StorageKey: "resource:resource-test"}}}
	if err := svc.prepareArkPrivateAssetReferences(context.Background(), admin.ID, &input); err != nil {
		t.Fatal(err)
	}
	if input.ReferenceImages[0].URL != "https://example.com/original.png" {
		t.Fatal("deleted asset was automatically recreated")
	}
}

func TestArkAssetAdminRejectsInvalidFiltersAndMalformedResult(t *testing.T) {
	for _, query := range []AdminArkAssetQuery{{Status: "unknown"}, {GroupType: "unknown"}, {Name: strings.Repeat("字", 65)}} {
		if _, err := arkAssetListPayload(query, "default"); err == nil {
			t.Fatalf("invalid query accepted: %+v", query)
		}
	}
	if err := decodeArkAssetResult(map[string]interface{}{}, &AdminArkAssetPage{}); err == nil {
		t.Fatal("missing Result accepted")
	}
}

func TestAdminArkAssetMutationRejectsWrongProject(t *testing.T) {
	mutations := 0
	svc, _, admin := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Action") != "GetAsset" {
			mutations++
		}
		_, _ = w.Write([]byte(`{"Result":{"Id":"asset-test","ProjectName":"other-project"}}`))
	})
	if err := svc.RenameAdminArkAsset(context.Background(), admin, "asset-test", AdminArkAssetRenameRequest{Name: "name"}); err == nil {
		t.Fatal("cross-project rename accepted")
	}
	if err := svc.DeleteAdminArkAsset(context.Background(), admin, "asset-test"); err == nil {
		t.Fatal("cross-project deletion accepted")
	}
	if mutations != 0 {
		t.Fatal("cross-project upstream mutation called")
	}
}

func TestAdminArkAssetDeletionPreservesPortraitLibraryAssets(t *testing.T) {
	deletes := 0
	svc, db, admin := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Action") == "GetAsset" {
			_, _ = w.Write([]byte(`{"Result":{"Id":"asset-test","ProjectName":"project-test"}}`))
			return
		}
		deletes++
		_, _ = w.Write([]byte(`{"Result":{}}`))
	})
	asset := &model.ArkPortraitAsset{ID: "portrait-record", ArkAssetID: "asset-test", Name: "已认证人像", Status: "active"}
	if err := db.Create(asset).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteAdminArkAsset(context.Background(), admin, "asset-test"); err == nil || !strings.Contains(err.Error(), "人像库") {
		t.Fatalf("portrait library guard = %v", err)
	}
	if deletes != 0 {
		t.Fatal("portrait library asset was deleted upstream")
	}
	if _, err := svc.repo.ArkPortraitAsset(asset.ID); err != nil {
		t.Fatalf("portrait record was removed: %v", err)
	}
}

func TestAdminArkAssetRenameSynchronizesPortraitLibraryName(t *testing.T) {
	failRename := true
	svc, db, admin := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Action") == "GetAsset" {
			_, _ = w.Write([]byte(`{"Result":{"Id":"asset-test","ProjectName":"project-test"}}`))
			return
		}
		if failRename {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"ResponseMetadata":{"Error":{"Code":"InternalError","Message":"unavailable"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"Result":{}}`))
	})
	asset := &model.ArkPortraitAsset{ID: "portrait-record", GroupID: "group-test", ArkAssetID: "asset-test", Name: "原名称", Status: "active"}
	if err := db.Create(asset).Error; err != nil {
		t.Fatal(err)
	}
	request := AdminArkAssetRenameRequest{Name: "新名称"}
	if err := svc.RenameAdminArkAsset(context.Background(), admin, "asset-test", request); err == nil {
		t.Fatal("upstream rename failure was swallowed")
	}
	stored, err := svc.repo.ArkPortraitAsset(asset.ID)
	if err != nil || stored.Name != "原名称" {
		t.Fatalf("local name changed after upstream failure: %+v %v", stored, err)
	}
	failRename = false
	if err := svc.RenameAdminArkAsset(context.Background(), admin, "asset-test", request); err != nil {
		t.Fatal(err)
	}
	stored, err = svc.repo.ArkPortraitAsset(asset.ID)
	if err != nil || stored.Name != "新名称" || stored.GroupID != "group-test" || stored.ArkAssetID != "asset-test" || stored.Status != "active" {
		t.Fatalf("portrait rename lost metadata: %+v %v", stored, err)
	}
}
