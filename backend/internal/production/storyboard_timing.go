package production

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	storyboardFPS       = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:fps|帧\s*[/／]\s*秒)`)
	storyboardFPSLabel  = regexp.MustCompile(`(?i)(?:帧率|fps)\s*[:：=]\s*(\d+(?:\.\d+)?)`)
	frameInterval       = regexp.MustCompile(`^\[\s*(\d+)\s*[,，]\s*(\d+)\s*\)\s*$`)
	secondsValue        = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s*(秒|s|ms|毫秒)?$`)
	timeRange           = regexp.MustCompile(`^\s*([0-9:.]+)\s*[–—−~～-]\s*([0-9:.]+)\s*[†‡*]?\s*$`)
	frameTimecodeFormat = regexp.MustCompile(`(?i)(?:HH:)?MM:SS:FF`)
)

type storyboardTiming struct {
	fps           float64
	frameTimecode bool
	seconds       float64
	roundedMs     int64
	frameEnd      *int64
	timeEnd       *float64
}

func (t *storyboardTiming) readFPS(text string) error {
	t.frameTimecode = frameTimecodeFormat.MatchString(text)
	matches := append(storyboardFPS.FindAllStringSubmatch(text, -1), storyboardFPSLabel.FindAllStringSubmatch(text, -1)...)
	for _, match := range matches {
		fps, _ := strconv.ParseFloat(match[1], 64)
		if fps <= 0 || fps > 240 {
			return fmt.Errorf("帧率必须大于 0 且不超过 240fps")
		}
		if t.fps != 0 && t.fps != fps {
			return fmt.Errorf("分镜包含不同帧率，请按集统一帧率")
		}
		t.fps = fps
	}
	return nil
}

func (t *storyboardTiming) rowSeconds(columns []storyboardColumn, cells []string) (float64, error) {
	var values []float64
	for i, column := range columns {
		cell := strings.TrimSpace(cells[i])
		if cell == "" || cell == "—" || cell == "-" {
			continue
		}
		var value float64
		var err error
		switch column {
		case columnDuration:
			value, err = parseStoryboardSeconds(cell)
		case columnFrames:
			if t.fps == 0 {
				return 0, fmt.Errorf("帧数需要明确帧率，请填写如“帧率：30fps”")
			}
			var frames int64
			frames, err = strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(cell, "帧")), 10, 64)
			value = float64(frames) / t.fps
		case columnFrameRange:
			if t.fps == 0 {
				return 0, fmt.Errorf("帧区间需要明确帧率")
			}
			match := frameInterval.FindStringSubmatch(cell)
			if match == nil {
				return 0, fmt.Errorf("帧区间应为 [起始帧, 结束帧)，末帧不包含")
			}
			start, e1 := strconv.ParseInt(match[1], 10, 64)
			end, e2 := strconv.ParseInt(match[2], 10, 64)
			if e1 != nil || e2 != nil || end <= start {
				return 0, fmt.Errorf("帧区间无效，结束帧必须大于起始帧")
			}
			if t.frameEnd != nil && start != *t.frameEnd {
				return 0, fmt.Errorf("帧区间与上一镜不连续，请检查重叠或缺帧")
			}
			t.frameEnd = &end
			value = float64(end-start) / t.fps
		case columnTimeRange:
			match := timeRange.FindStringSubmatch(cell)
			if match == nil {
				return 0, fmt.Errorf("时间码应为“起点–出点”，支持 MM:SS、HH:MM:SS 或明确声明的帧时间码")
			}
			start, e1 := t.parseTimecode(match[1], false)
			end, e2 := t.parseTimecode(match[2], false)
			if e1 != nil {
				return 0, e1
			}
			if e2 != nil {
				return 0, e2
			}
			if end <= start {
				return 0, fmt.Errorf("时间码出点必须晚于入点")
			}
			if t.timeEnd != nil && math.Abs(start-*t.timeEnd) > 0.001 {
				return 0, fmt.Errorf("时间码与上一镜不连续，请检查重叠或空隙")
			}
			t.timeEnd = &end
			value = end - start
		default:
			continue
		}
		if err != nil {
			return 0, err
		}
		if value <= 0 {
			return 0, fmt.Errorf("已填写的镜头时长或帧数必须大于 0")
		}
		if err := validateImportedSeconds(value); err != nil {
			return 0, err
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return 0, nil
	}
	for _, value := range values[1:] {
		if math.Abs(value-values[0]) > 0.001 {
			return 0, fmt.Errorf("时长、时间码、帧区间或帧数不一致，请核对原稿")
		}
	}
	return values[0], nil
}

func parseStoryboardSeconds(cell string) (float64, error) {
	if strings.Contains(cell, ":") {
		return (&storyboardTiming{}).parseTimecode(cell, true)
	}
	match := secondsValue.FindStringSubmatch(cell)
	if match == nil {
		return 0, fmt.Errorf("时长格式无效，请填写秒数或 MM:SS 时间")
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if strings.EqualFold(match[2], "ms") || match[2] == "毫秒" {
		value /= 1000
	}
	return value, err
}

// 三段时间码默认 HH:MM:SS；MM:SS:FF 必须在文档中明确声明，不能靠数字大小猜测。
func (t *storyboardTiming) parseTimecode(cell string, clockOnly bool) (float64, error) {
	parts := strings.Split(cell, ":")
	frameCode := !clockOnly && t.frameTimecode
	if len(parts) < 2 || len(parts) > 4 || (len(parts) == 4 && !frameCode) {
		return 0, fmt.Errorf("无法识别时间码 %s", cell)
	}
	if frameCode && (len(parts) < 3 || t.fps != math.Trunc(t.fps)) {
		return 0, fmt.Errorf("帧时间码需使用 MM:SS:FF 或 HH:MM:SS:FF；分数帧率请使用帧数或帧区间，避免混淆丢帧与非丢帧计时")
	}
	var value float64
	for i, part := range parts {
		n, err := strconv.ParseFloat(part, 64)
		if err != nil || math.IsInf(n, 0) || n < 0 {
			return 0, fmt.Errorf("时间码无效：%s", cell)
		}
		if frameCode && i == len(parts)-1 {
			if t.fps == 0 || n != math.Trunc(n) || n >= math.Ceil(t.fps) {
				return 0, fmt.Errorf("时间码帧位超出帧率或缺少帧率：%s", cell)
			}
			return value + n/t.fps, nil
		}
		if i > 0 && n >= 60 || i < len(parts)-1 && n != math.Trunc(n) {
			return 0, fmt.Errorf("时间码分秒位无效：%s", cell)
		}
		value = value*60 + n
	}
	return value, nil
}

// 按累计边界取整，而非逐镜取整，避免非整毫秒帧时长在整集累积漂移。
func (t *storyboardTiming) roundDuration(seconds float64) float64 {
	if seconds == 0 {
		return 0
	}
	t.seconds += seconds
	end := int64(math.Round(t.seconds * 1000))
	duration := end - t.roundedMs
	t.roundedMs = end
	return float64(duration) / 1000
}
