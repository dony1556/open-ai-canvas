package production

import (
	"fmt"
	"strings"
	"testing"
)

func parseImportedForTest(t *testing.T, source string) []ScriptShot {
	t.Helper()
	shots, recognized, err := ParseStoryboardSource(source)
	if !recognized || err != nil {
		t.Fatalf("recognized=%v err=%v", recognized, err)
	}
	return shots
}

func TestStoryboardFrameTimelineAndSoundNotes(t *testing.T) {
	// Synthetic content with the same production-table structure, not tied to a project or title.
	frames := []int{3, 100, 42, 21, 53, 22, 37, 40, 56, 18, 65, 20, 30, 35, 45, 50, 35, 40, 68}
	var source strings.Builder
	source.WriteString("帧率：30 fps，时间码 MM:SS:FF。\n\n| 镜号 | 原片入点–出点 | 测试帧区间 | 帧数 | 构图及机位 | 表演、运动、衔接与合成 | 对白与声音 |\n|---|---|---|---|---|---|---|\n")
	tc := func(frame int) string { return fmt.Sprintf("%02d:%02d:%02d", frame/1800, frame/30%60, frame%30) }
	cursor := 0
	for i, n := range frames {
		sound := "动作声待听校"
		if i == 0 {
			sound = "无可见字幕"
		}
		if i == 1 {
			sound = "保安 · 走后门。"
		}
		fmt.Fprintf(&source, "| S%02d | %s–%s | [%d, %d) | %d | 街道近景 | 角色走向门口 | %s |\n", i+1, tc(1320+cursor), tc(1320+cursor+n), cursor, cursor+n, n, sound)
		cursor += n
	}
	source.WriteString("\n## 其他说明\n不是分镜行\n")
	shots := parseImportedForTest(t, source.String())
	if len(shots) != 19 || cursor != 780 {
		t.Fatalf("shots=%d frames=%d", len(shots), cursor)
	}
	var total int64
	for i, shot := range shots {
		total += (StoryboardSettings{}).ImportedShotMs(shot.DurationSeconds, shot.Dialogue)
		if shot.Action != "角色走向门口" || !strings.Contains(shot.ImagePrompt, "街道近景") {
			t.Fatalf("lost direction at %d: %+v", i, shot)
		}
		if i != 1 && (shot.Dialogue != "" || !strings.Contains(shot.VideoPrompt, "声音备注")) {
			t.Fatalf("sound became dialogue: %+v", shot)
		}
	}
	if total != 26000 || shots[0].DurationSeconds != 0.1 {
		t.Fatalf("total=%d first=%v", total, shots[0].DurationSeconds)
	}
	if shots[1].Speaker != "保安" || shots[1].Dialogue != "走后门。" {
		t.Fatalf("speaker: %+v", shots[1])
	}
}

func TestStoryboardFrameRatesAndClockDurations(t *testing.T) {
	for _, fps := range []int{24, 25, 30, 60} {
		t.Run(fmt.Sprint(fps), func(t *testing.T) {
			var source strings.Builder
			fmt.Fprintf(&source, "帧率：%d\n| 帧数 | 画面 |\n|---|---|\n", fps)
			for i := 0; i < fps; i++ {
				source.WriteString("| 1 | 转身 |\n")
			}
			shots := parseImportedForTest(t, source.String())
			var total int64
			for _, shot := range shots {
				total += (StoryboardSettings{}).ImportedShotMs(shot.DurationSeconds, "")
			}
			if total != 1000 {
				t.Fatalf("frame rounding drift: %d", total)
			}
		})
	}
	for _, value := range []string{"2.5秒", "2500ms", "00:02.5", "00:00:02.5"} {
		shots := parseImportedForTest(t, "画面\t时长\n转身\t"+value)
		if shots[0].DurationSeconds != 2.5 {
			t.Fatalf("%s: %+v", value, shots)
		}
	}
	shots := parseImportedForTest(t, "帧率：29.97fps\n| 画面 | 帧数 |\n|---|---|\n| 转身 | 30 |")
	if shots[0].DurationSeconds != 1.001 {
		t.Fatalf("fractional frame rate: %+v", shots)
	}
	shots = parseImportedForTest(t, "| 画面 | 原片入点–出点 |\n|---|---|\n| 转身 | 01:02:03.5–01:02:05 |")
	if shots[0].DurationSeconds != 1.5 {
		t.Fatalf("HH:MM:SS: %+v", shots)
	}
}

func TestStoryboardInvalidTimingAndAmbiguousDialogueAreErrors(t *testing.T) {
	cases := []string{
		"| 画面 | 帧数 |\n|---|---|\n| 转身 | 3 |",
		"30fps\n| 画面 | 帧数 | 测试帧区间 |\n|---|---|---|\n| 转身 | 4 | [0,3) |",
		"30fps\n| 画面 | 测试帧区间 |\n|---|---|\n| 转身 | [0,3) |\n| 转身 | [4,6) |",
		"30fps MM:SS:FF\n| 画面 | 原片入点–出点 |\n|---|---|\n| 转身 | 00:00:00–00:00:30 |",
		"30fps MM:SS:FF\n| 画面 | 原片入点–出点 |\n|---|---|\n| 转身 | 00:00–00:01 |",
		"29.97fps MM:SS:FF\n| 画面 | 原片入点–出点 |\n|---|---|\n| 转身 | 00:00:00–00:01:00 |",
		"30fps 24fps\n| 画面 | 帧数 |\n|---|---|\n| 转身 | 3 |",
		"| 画面 | 时长 |\n|---|---|\n| 转身 | 不知道 |",
		"| 画面 | 时长 |\n|---|---|\n| 转身 | 0 |",
		"| 画面 | 时长 |\n|---|---|\n| 转身 | 61秒 |",
		"| 画面 | 时长 |\n|---|---|\n| 转身 | 1 | 多一列 |",
		"| 画面 | 对白与声音 |\n|---|---|\n| 转身 | 保安 · Go back.／走后门。 |",
		"| 画面 | 对白与声音 |\n|---|---|\n| 转身 | 没有标注的内容 |",
		`{"shots":[{"action":"转身","durationSeconds":-1}]}`,
		`{"shots":[{"action":"转身","durationSeconds":0.0001}]}`,
		`{"shots":[{"action":"转身","durationSeconds":2}]`,
	}
	for i, source := range cases {
		shots, recognized, err := ParseStoryboardSource(source)
		if !recognized || err == nil || len(shots) > 0 {
			t.Errorf("case %d must stop import, got recognized=%v err=%v shots=%v", i, recognized, err, shots)
		}
	}
}

func TestStoryboardExistingFormatsAndExplicitTiming(t *testing.T) {
	shots := parseImportedForTest(t, "| 角色 | 台词 | 画面 | 时长 | 音效 | 翻译 |\n|---|---|---|---|---|---|\n| 小明 | （轻声）你好<br>再见 | 挥手 | 1秒 | 门响 | Hello |")
	if len(shots) != 2 || shots[0].DurationSeconds != 0.5 || shots[1].DurationSeconds != 0.5 || shots[0].Dialogue != "你好" || !strings.Contains(shots[0].Action, "轻声") {
		t.Fatalf("legacy table: %+v", shots)
	}
	for _, shot := range shots {
		if (StoryboardSettings{}).ImportedShotMs(shot.DurationSeconds, shot.Dialogue) != 500 {
			t.Fatal("explicit timing stretched")
		}
	}
	shots = parseImportedForTest(t, `{"shots":[{"action":"转身","durationSeconds":0.1,"dialogue":"有台词也不能覆盖原稿时间"}]}`)
	if (StoryboardSettings{}).ImportedShotMs(shots[0].DurationSeconds, shots[0].Dialogue) != 100 {
		t.Fatal("JSON timing stretched")
	}
	shots = parseImportedForTest(t, "| 画面 | 台词 |\n|---|---|\n| 转身 | 无 |")
	if shots[0].DurationSeconds != 0 || (StoryboardSettings{}).ImportedShotMs(0, "") != DefaultShotMs {
		t.Fatal("missing timing must still estimate")
	}
	if _, ok, err := ParseStoryboardSource("普通剧本文本，没有分镜表"); ok || err != nil {
		t.Fatal("unrecognized source should allow transcription")
	}
}

func TestStoryboardMultipleTablesDoNotLoseLaterChapters(t *testing.T) {
	source := "24fps\n| 画面 | 测试帧区间 |\n|---|---|\n| 第一章动作 | [0,24) |\n\n## 第二章\n\n| 画面 | 时长 |\n|---|---|\n| 第二章动作 | 2秒 |"
	shots := parseImportedForTest(t, source)
	if len(shots) != 2 || shots[0].DurationSeconds != 1 || shots[1].DurationSeconds != 2 {
		t.Fatalf("lost later table: %+v", shots)
	}
}
