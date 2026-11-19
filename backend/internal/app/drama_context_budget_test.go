package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestDramaTextContextSendsReservedOutputLimit(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	svc, db, _, _ := creationTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.MaxTokens != 8000 {
			t.Errorf("upstream max_tokens = %d, want 8000", body.MaxTokens)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	t.Cleanup(server.Close)
	if err := db.Model(&model.ModelChannel{}).Where("id = ?", "channel").Updates(map[string]any{"base_url": server.URL, "api_key": "test"}).Error; err != nil {
		t.Fatal(err)
	}
	input := creationTextRequest().Input
	input["metadata"] = map[string]any{"source": "drama_production"}
	input["textOptions"] = map[string]any{"stream": false}
	if _, err := dramaTextContextReport(input, 32000, 8000); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.processCanvasGenerationTask(context.Background(), "user", "", "canvas_text", "", string(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestDramaTextContextBudgetsEntireRequest(t *testing.T) {
	input := func() map[string]any {
		return map[string]any{"prompt": strings.Repeat("中", 20000), "config": map[string]any{"systemPrompt": strings.Repeat("规则", 1000)}}
	}
	large := input()
	report, err := dramaTextContextReport(large, 128000, 16000)
	if err != nil || !strings.Contains(report, "128000") {
		t.Fatalf("large model should accept long skills: %s %v", report, err)
	}
	if intFromAny(large["textOptions"].(map[string]any)["maxOutputTokens"]) != 16000 {
		t.Fatal("output reservation differs from request")
	}
	if _, err := dramaTextContextReport(input(), 32000, 8000); err == nil {
		t.Fatal("entire request exceeds small model")
	}
	unknown := input()
	report, err = dramaTextContextReport(unknown, 0, 0)
	if err != nil || !strings.Contains(report, "容量未知") || unknown["prompt"] != input()["prompt"] {
		t.Fatal("unknown capacity must warn without inventing or cutting a limit")
	}
	explicit := map[string]any{"prompt": "短文", "textOptions": map[string]any{"maxOutputTokens": 1024}}
	if _, err := dramaTextContextReport(explicit, 8000, 4096); err != nil {
		t.Fatal(err)
	}
	if _, err := dramaTextContextReport(explicit, 8000, 512); err == nil {
		t.Fatal("overlarge output accepted")
	}
}

func TestDramaTextContextUsesSmallestLogicalRoute(t *testing.T) {
	cap := DefaultModelCapabilityConfigForModel(string(model.ChannelInterfaceChatCompletion), "text-test")
	first := *cap.Text
	first.ContextWindowTokens = 128000
	first.MaxOutputTokens = 16000
	second := first
	second.ContextWindowTokens = 32000
	second.MaxOutputTokens = 8000
	s := &Service{repo: repository.New(nil), routeCatalogTTL: time.Hour, routeCatalog: &routeCatalogSnapshot{LoadedAt: time.Now(), Models: map[string]cachedLogicalModel{"logical": {Routes: []cachedLogicalRoute{
		{CapabilitySpec: CapabilitySpec{Capability: "text"}, ChannelModel: model.ChannelModel{Capability: "text", CapabilityConfigJSON: mustEncodeModelCapabilityConfig(t, &ModelCapabilityConfig{Version: 1, Text: &first})}},
		{CapabilitySpec: CapabilitySpec{Capability: "text"}, ChannelModel: model.ChannelModel{Capability: "text", CapabilityConfigJSON: mustEncodeModelCapabilityConfig(t, &ModelCapabilityConfig{Version: 1, Text: &second})}},
	}}}}}
	w, o := s.dramaTextContextLimits(DramaModelChoice{LogicalModelID: "logical"})
	if w != 32000 || o != 8000 {
		t.Fatalf("unsafe intersection: %d %d", w, o)
	}
	if w, _ := s.dramaTextContextLimits(DramaModelChoice{LogicalModelID: "missing"}); w != 0 {
		t.Fatal("unknown model received invented window")
	}
}

func TestDramaTextContextRejectsBeforeTaskSubmission(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	called := false
	s.dramaTaskHook = func(string, CreateTaskRequest, *renderProject) (*model.Task, error) { called = true; return nil, nil }
	row, err := s.repo.ChannelModelByKey("channel", "text-test")
	if err != nil {
		t.Fatal(err)
	}
	cap := DefaultModelCapabilityConfigForModel(string(model.ChannelInterfaceChatCompletion), "text")
	cap.Text.ContextWindowTokens = 4096
	cap.Text.MaxOutputTokens = 1024
	row.CapabilityConfigJSON = mustEncodeModelCapabilityConfig(t, cap)
	if err := db.Save(row).Error; err != nil {
		t.Fatal(err)
	}
	run := &dramaRun{s: s, project: &model.Project{ID: "budget-test", UserID: "usr-drama"}}
	_, err = run.createTaskWith(&model.DramaJob{}, DramaModelChoice{ChannelID: "channel", ModelKey: "text-test"}, "text", "text", strings.Repeat("正文", 4000), map[string]any{"config": map[string]any{"systemPrompt": "资产提取"}})
	if err == nil || !strings.Contains(err.Error(), "上下文预算") || called {
		t.Fatalf("budget check ran after task submission: %v", err)
	}
}
