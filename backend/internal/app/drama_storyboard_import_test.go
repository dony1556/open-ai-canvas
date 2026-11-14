package app

import (
	"math"
	"strings"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/production"
)

func TestDramaDirectStoryboardPreservesTimingAndStopsInvalidImports(t *testing.T) {
	for _, tc := range []struct{ name, source, wantError string }{
		{"timed", "30fps MM:SS:FF\n| 镜号 | 测试帧区间 | 帧数 | 构图及机位 | 表演、运动、衔接与合成 | 对白与声音 |\n|---|---|---|---|---|---|\n| S1 | [0,3) | 3 | 门口近景 | 角色抬头 | 无可见字幕 |\n| S2 | [3,33) | 30 | 门口近景 | 角色转身 | 配乐待核 |\n| S3 | [33,60) | 27 | 门口近景 | 角色离开 | 动作声待听校 |", ""},
		{"missingFPS", "| 画面 | 帧数 |\n|---|---|\n| 转身 | 3 |", "帧率"},
		{"conflict", "30fps\n| 画面 | 帧数 | 测试帧区间 |\n|---|---|---|\n| 转身 | 4 | [0,3) |", "不一致"},
		{"bilingual", "| 画面 | 对白与声音 |\n|---|---|\n| 转身 | 小明 · Hello.／你好。 |", "双语"},
		{"overModelLimit", "| 画面 | 时长 |\n|---|---|\n| 转身 | 11秒 |", "超过视频模型"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, fake := newDramaTestService(t)
			settings := DramaSettings{Models: fakeDramaModels}
			settings.EpisodeSeconds = 30
			summary, err := svc.CreateDrama("usr-drama", CreateDramaRequest{Name: tc.name, SourceFormat: production.SourceStoryboard, Settings: settings, Chapters: []DramaChapterInput{{Title: "第一章", Text: tc.source}}})
			if err != nil {
				t.Fatal(err)
			}
			episodes, err := svc.repo.DramaEpisodes(summary.Project.ID)
			if err != nil || len(episodes) != 1 {
				t.Fatalf("episodes=%v err=%v", episodes, err)
			}
			row, err := svc.repo.DramaProduction(summary.Project.ID)
			if err != nil {
				t.Fatal(err)
			}
			run := &dramaRun{s: svc, production: row, project: &summary.Project, settings: parseDramaSettings(row.SettingsJSON)}
			handled, err := run.applyDirectStoryboard(&episodes[0])
			if !handled || err != nil {
				t.Fatalf("handled=%v err=%v", handled, err)
			}
			updated, err := svc.repo.DramaEpisodes(summary.Project.ID)
			if err != nil {
				t.Fatal(err)
			}
			shots, err := svc.repo.DramaShots(episodes[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" {
				if len(shots) != 0 || !strings.Contains(updated[0].Error, tc.wantError) {
					t.Fatalf("invalid import persisted shots=%v episode=%+v", shots, updated[0])
				}
			} else {
				if len(shots) != 3 || updated[0].DurationMs != 2000 || shots[0].DurationMs != 100 || shots[0].Dialogue != "" || shots[0].Action != "角色抬头" || !strings.Contains(shots[0].ImagePrompt, "门口近景") {
					t.Fatalf("shots=%+v episode=%+v", shots, updated[0])
				}
				seconds := 0.033
				shot, err := svc.UpdateDramaShot("usr-drama", summary.Project.ID, shots[0].ID, UpdateDramaShotRequest{DurationSeconds: &seconds})
				if err != nil || shot.DurationMs != 33 {
					t.Fatalf("subsecond edit: %+v %v", shot, err)
				}
				for _, bad := range []float64{0, 0.0001, 61, math.NaN(), math.Inf(1)} {
					if _, err := svc.UpdateDramaShot("usr-drama", summary.Project.ID, shots[0].ID, UpdateDramaShotRequest{DurationSeconds: &bad}); err == nil {
						t.Fatalf("accepted invalid duration %v", bad)
					}
				}
			}
			var tasks int64
			fake.db.Model(&model.Task{}).Count(&tasks)
			if tasks != 0 {
				t.Fatalf("direct import must not invoke paid generation: %d", tasks)
			}
		})
	}
}
