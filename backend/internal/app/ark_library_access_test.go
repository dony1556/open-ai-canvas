package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"yingce/backend/internal/model"
)

func TestArkLibraryVisibilityCoversEntireRemoteLibraryAndGroups(t *testing.T) {
	calls := 0
	svc, _, admin := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["ProjectName"] != "project-test" {
			t.Fatal("unexpected project")
		}
		switch r.URL.Query().Get("Action") {
		case "ListAssets":
			_, _ = w.Write([]byte(`{"Result":{"Items":[{"Id":"asset-first"},{"Id":"asset-other-user"}],"NextToken":"next"}}`))
		case "ListAssetGroups":
			if payload["MaxResults"] != float64(100) {
				t.Fatal("groups pagination missing")
			}
			_, _ = w.Write([]byte(`{"Result":{"Items":[{"Id":"group-existing","Name":"已上传素材","GroupType":"AIGC"}],"NextToken":"more-groups"}}`))
		default:
			t.Fatal("visibility must only read remote data")
		}
	})
	ctx := context.Background()
	viewer := &model.User{ID: "viewer", Role: model.UserRoleUser}
	checkDenied := func(user *model.User) {
		t.Helper()
		before := calls
		if _, err := svc.ArkExistingAssets(ctx, user, AdminArkAssetQuery{}); err == nil {
			t.Fatal("assets access unexpectedly allowed")
		}
		if _, err := svc.ArkExistingGroups(ctx, user, AdminArkAssetQuery{}); err == nil {
			t.Fatal("groups access unexpectedly allowed")
		}
		if calls != before {
			t.Fatal("denied account reached upstream")
		}
	}
	checkDenied(nil)
	checkDenied(viewer)
	checkAll := func(user *model.User) {
		t.Helper()
		page, err := svc.ArkExistingAssets(ctx, user, AdminArkAssetQuery{})
		if err != nil || len(page.Items) != 2 || page.NextToken != "next" {
			t.Fatalf("whole-library page=%+v err=%v", page, err)
		}
		groups, err := svc.ArkExistingGroups(ctx, user, AdminArkAssetQuery{})
		if err != nil || len(groups.Items) != 1 || groups.NextToken != "more-groups" {
			t.Fatalf("groups=%+v err=%v", groups, err)
		}
	}
	checkAll(admin)
	record, setting, err := svc.readArkPrivateAssetSetting()
	if err != nil {
		t.Fatal(err)
	}
	setting.LibraryVisibleUserIDs = []string{viewer.ID}
	if err := svc.saveArkPrivateAssetSetting(record, setting); err != nil {
		t.Fatal(err)
	}
	checkAll(viewer)
	checkDenied(&model.User{ID: "unselected"})
	setting.LibraryVisibleToAll = true
	if err := svc.saveArkPrivateAssetSetting(record, setting); err != nil {
		t.Fatal(err)
	}
	checkAll(&model.User{ID: "any-user"})
	checkDenied(&model.User{})
	setting.LibraryVisibleToAll = false
	setting.LibraryVisibleUserIDs = nil
	if err := svc.saveArkPrivateAssetSetting(record, setting); err != nil {
		t.Fatal(err)
	}
	checkDenied(viewer)
	checkAll(admin)
}

func TestArkLibraryVisibilitySettingsValidateAndPreserveSecret(t *testing.T) {
	svc, db, admin := newAdminArkAssetTestService(t, func(http.ResponseWriter, *http.Request) { t.Error("settings must not call upstream") })
	if err := db.AutoMigrate(&model.User{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{ID: "viewer", Username: "viewer", Role: model.UserRoleUser, Status: model.UserStatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	req := ArkPrivateAssetSettingRequest{Region: "cn-beijing", ProjectName: "project-test", AccessKeyID: "test-ak", LibraryVisibleUserIDs: []string{"viewer", "viewer"}}
	view, err := svc.UpdateArkPrivateAssetSetting(admin, req)
	if err != nil {
		t.Fatal(err)
	}
	if !view.HasAccessKeySecret || view.LibraryVisibleToAll || len(view.LibraryVisibleUserIDs) != 1 {
		t.Fatalf("saved visibility=%+v", view)
	}
	_, setting, err := svc.readArkPrivateAssetSetting()
	if err != nil {
		t.Fatal(err)
	}
	if setting.AccessKeySecret != "test-sk" || len(setting.LibraryVisibleUserIDs) != 1 {
		t.Fatal("visibility update damaged credentials")
	}
	req.LibraryVisibleUserIDs = []string{"does-not-exist"}
	if _, err := svc.UpdateArkPrivateAssetSetting(admin, req); err == nil {
		t.Fatal("unknown user accepted")
	}
	_, after, _ := svc.readArkPrivateAssetSetting()
	if len(after.LibraryVisibleUserIDs) != 1 || after.LibraryVisibleUserIDs[0] != "viewer" {
		t.Fatal("failed settings write changed access")
	}
	req.LibraryVisibleUserIDs = nil
	req.LibraryVisibleToAll = true
	view, err = svc.UpdateArkPrivateAssetSetting(admin, req)
	if err != nil || !view.LibraryVisibleToAll {
		t.Fatalf("allow all update=%+v %v", view, err)
	}
	if _, err := svc.UpdateArkPrivateAssetSetting(&model.User{ID: "viewer"}, req); err == nil {
		t.Fatal("ordinary user changed library policy")
	}
	req.LibraryVisibleUserIDs = strings.Fields(strings.Repeat("user ", 501))
	for i := range req.LibraryVisibleUserIDs {
		req.LibraryVisibleUserIDs[i] = newID()
	}
	if _, err := arkPrivateAssetSettingFromRequest(req, setting); err == nil {
		t.Fatal("unbounded user list accepted")
	}
}

func TestArkLibraryGroupsPreservesTypeAndCursor(t *testing.T) {
	calls := 0
	svc, _, admin := newAdminArkAssetTestService(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		filter := payload["Filter"].(map[string]interface{})
		if filter["GroupType"] != "LivenessFace" || payload["NextToken"] != "group-cursor" || payload["MaxResults"] != float64(100) {
			t.Fatalf("group payload=%+v", payload)
		}
		_, _ = w.Write([]byte(`{"Result":{"Items":[],"NextToken":"next-group"}}`))
	})
	page, err := svc.ArkExistingGroups(context.Background(), admin, AdminArkAssetQuery{GroupType: "LivenessFace", NextToken: "group-cursor"})
	if err != nil || len(page.Items) != 0 || page.NextToken != "next-group" {
		t.Fatalf("groups=%+v error=%v", page, err)
	}
	if _, err := svc.ArkExistingGroups(context.Background(), admin, AdminArkAssetQuery{GroupType: "invalid"}); err == nil {
		t.Fatal("invalid group type accepted")
	}
	if calls != 1 {
		t.Fatal("invalid filter reached remote API")
	}
}
