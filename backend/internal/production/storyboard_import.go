package production

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// ParseStoryboardSource 直接解析用户写好的分镜，不经过文本模型，台词与镜头一字不改。
// 支持两种格式：与剧本输出同结构的 JSON（{"shots":[...]} 或镜头数组），
// 以及带表头的 Markdown 表格 / 制表符分隔文本。未识别返回 false；已识别但有冲突则报错，禁止回退模型猜测。
func ParseStoryboardSource(text string) ([]ScriptShot, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, false, nil
	}
	if shots, ok, err := parseStoryboardJSON(text); ok {
		return shots, true, err
	}
	return parseStoryboardTable(text)
}

func parseStoryboardJSON(text string) ([]ScriptShot, bool, error) {
	if strings.HasPrefix(text, "[") {
		text = `{"shots":` + text + `}`
	}
	candidate, err := ExtractJSONObject(text)
	if err != nil {
		if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "```json") {
			return nil, true, fmt.Errorf("分镜 JSON 无法解析：%w", err)
		}
		return nil, false, nil
	}
	var payload struct {
		Shots []ScriptShot `json:"shots"`
	}
	if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
		return nil, true, fmt.Errorf("分镜 JSON 无效：%w", err)
	}
	if len(payload.Shots) == 0 {
		return nil, false, nil
	}
	if len(payload.Shots) > MaxScriptShots {
		return nil, true, fmt.Errorf("单集分镜不能超过 %d 镜", MaxScriptShots)
	}
	for i, shot := range payload.Shots {
		if err := validateImportedSeconds(shot.DurationSeconds); err != nil {
			return nil, true, fmt.Errorf("第 %d 镜：%w", i+1, err)
		}
	}
	shots, err := normalizeScriptShots(payload.Shots)
	if err == nil && len(shots) != len(payload.Shots) {
		return nil, true, fmt.Errorf("分镜包含缺少画面、动作和对白的空镜头，请补全后导入")
	}
	return shots, true, err
}

type storyboardColumn int

const (
	columnIgnore storyboardColumn = iota
	columnScene
	columnCharacters
	columnProps
	columnSpeaker
	columnDialogue
	columnNarration
	columnAction
	columnImage
	columnShotSize
	columnCamera
	columnDuration
	columnTransition
	columnFrames
	columnFrameRange
	columnTimeRange
	columnSound
	columnDialogueSound
	columnTranslation
)

// storyboardHeader 按表头关键字识别列；先判断台词类，避免“角色台词”被当成出场角色。
func storyboardHeader(cell string) storyboardColumn {
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(cell), " ", ""))
	has := func(keys ...string) bool {
		for _, key := range keys {
			if strings.Contains(value, key) {
				return true
			}
		}
		return false
	}
	switch {
	case value == "":
		return columnIgnore
	case has("翻译", "translation"):
		return columnTranslation
	case has("帧区间", "帧范围", "framerange"):
		return columnFrameRange
	case has("帧数", "framecount"):
		return columnFrames
	case has("入点", "出点", "时间码", "timecode"):
		return columnTimeRange
	case has("对白", "台词", "对话", "dialogue") && has("声音", "音效", "声效", "sound", "audio"):
		return columnDialogueSound
	case has("台词", "对白", "对话", "dialogue") || value == "line" || value == "lines":
		return columnDialogue
	case has("旁白", "画外音", "narration"):
		return columnNarration
	case has("说话人", "讲话人", "speaker"):
		return columnSpeaker
	case has("声音", "音效", "声效", "配乐", "sound", "audio"):
		return columnSound
	case has("景别", "shotsize"):
		return columnShotSize
	case has("画面", "构图", "image", "visual"):
		return columnImage
	case has("表演", "合成", "动作", "内容", "描述", "剧情", "action", "description"):
		return columnAction
	case has("运镜", "镜头运动", "机位", "camera"):
		return columnCamera
	case has("时长", "秒", "duration", "time"):
		return columnDuration
	case has("转场", "transition"):
		return columnTransition
	case has("场景", "地点", "scene", "location"):
		return columnScene
	case has("道具", "prop"):
		return columnProps
	case has("角色", "人物", "出场", "character"):
		return columnCharacters
	}
	return columnIgnore
}

var (
	markdownRule  = regexp.MustCompile(`^:?-{2,}:?$`)
	speakerPrefix = regexp.MustCompile(`^\s*([^：:“”"「」（）()\s，。！？,.!?]{1,10})\s*(?:（[^）]*）|\([^)]*\))?\s*[：:]\s*(.+)$`)
	htmlBreak     = regexp.MustCompile(`(?i)<br\s*/?>`)
	leadingCue    = regexp.MustCompile(`^\s*(?:（[^）]*）|\([^)]*\))`)
)

func splitStoryboardCells(line string, pipe bool) []string {
	if pipe {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "|")
		line = strings.TrimSuffix(line, "|")
		cells := strings.Split(line, "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		return cells
	}
	cells := strings.Split(line, "\t")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func parseStoryboardTable(text string) ([]ScriptShot, bool, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	pipe := strings.Count(text, "|") >= 4
	if !pipe && !strings.Contains(text, "\t") {
		return nil, false, nil
	}
	timing := storyboardTiming{}
	fpsErr := timing.readFPS(text)
	var columns []storyboardColumn
	shots := []ScriptShot{}
	for lineIndex, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(shots) > 0 {
				columns = nil
				timing.frameEnd, timing.timeEnd = nil, nil
			}
			continue
		}
		if pipe && !strings.Contains(line, "|") {
			if len(shots) > 0 {
				columns = nil
				timing.frameEnd, timing.timeEnd = nil, nil
			}
			continue
		}
		if !pipe && !strings.Contains(line, "\t") {
			if len(shots) > 0 {
				columns = nil
				timing.frameEnd, timing.timeEnd = nil, nil
			}
			continue
		}
		cells := splitStoryboardCells(line, pipe)
		if columns == nil {
			mapped := make([]storyboardColumn, len(cells))
			found := map[storyboardColumn]bool{}
			for i, cell := range cells {
				mapped[i] = storyboardHeader(cell)
				found[mapped[i]] = true
			}
			if found[columnDialogue] || found[columnDialogueSound] || found[columnAction] || found[columnImage] {
				if fpsErr != nil {
					return nil, true, fpsErr
				}
				if !found[columnAction] {
					// 只有“画面”一列时它就是镜头内容。
					for i := range mapped {
						if mapped[i] == columnImage {
							mapped[i] = columnAction
							break
						}
					}
				}
				columns = mapped
			} else if len(shots) > 0 {
				return nil, true, fmt.Errorf("分镜第 %d 行：后续表格缺少可识别表头，请补全后导入", lineIndex+1)
			}
			continue
		}
		if isMarkdownRule(cells) {
			continue
		}
		if len(cells) != len(columns) {
			return nil, true, fmt.Errorf("分镜第 %d 行：列数与表头不一致", lineIndex+1)
		}
		seconds, err := timing.rowSeconds(columns, cells)
		if err != nil {
			return nil, true, fmt.Errorf("分镜第 %d 行：%w", lineIndex+1, err)
		}
		row, err := storyboardRow(columns, cells)
		if err != nil {
			return nil, true, fmt.Errorf("分镜第 %d 行：%w", lineIndex+1, err)
		}
		for i := range row {
			if seconds > 0 && seconds/float64(len(row)) < 0.001 {
				return nil, true, fmt.Errorf("分镜第 %d 行：拆分台词后单镜不足 1 毫秒，请调整分镜", lineIndex+1)
			}
			row[i].DurationSeconds = timing.roundDuration(seconds / float64(len(row)))
		}
		shots = append(shots, row...)
		if len(shots) > MaxScriptShots {
			return nil, true, fmt.Errorf("单集分镜不能超过 %d 镜", MaxScriptShots)
		}
	}
	if len(shots) == 0 {
		return nil, false, nil
	}
	normalized, err := normalizeScriptShots(shots)
	if err == nil && len(normalized) != len(shots) {
		return nil, true, fmt.Errorf("分镜包含缺少画面、动作和对白的空镜头，请补全后导入")
	}
	return normalized, true, err
}

func isMarkdownRule(cells []string) bool {
	for _, cell := range cells {
		if cell != "" && !markdownRule.MatchString(cell) {
			return false
		}
	}
	return true
}

// storyboardRow 把一行表格转成镜头；一格里有多句台词（<br> 分隔）时拆成连续镜头，每镜一句。
func storyboardRow(columns []storyboardColumn, cells []string) ([]ScriptShot, error) {
	var shot ScriptShot
	dialogue, narration := "", ""
	for i, cell := range cells {
		if i >= len(columns) || cell == "" {
			continue
		}
		switch columns[i] {
		case columnScene:
			shot.Scene = cell
		case columnCharacters:
			shot.Characters = splitNames(cell)
		case columnProps:
			shot.Props = splitNames(cell)
		case columnSpeaker:
			shot.Speaker = cell
		case columnDialogue:
			dialogue = cell
		case columnDialogueSound:
			var err error
			var notes string
			dialogue, notes, err = storyboardSoundCell(cell)
			if err != nil {
				return nil, err
			}
			if notes != "" {
				shot.VideoPrompt = joinField(shot.VideoPrompt, notes)
			}
		case columnSound:
			shot.VideoPrompt = joinField(shot.VideoPrompt, "声音备注："+cell)
		case columnTranslation:
			shot.VideoPrompt = joinField(shot.VideoPrompt, "翻译参考（不发音）："+cell)
		case columnNarration:
			narration = cell
		case columnAction:
			shot.Action = joinField(shot.Action, htmlBreak.ReplaceAllString(cell, "；"))
		case columnImage:
			shot.ImagePrompt = joinField(shot.ImagePrompt, htmlBreak.ReplaceAllString(cell, "；"))
		case columnShotSize:
			shot.ShotSize = cell
		case columnCamera:
			shot.Camera = cell
		case columnTransition:
			shot.Transition = cell
		}
	}
	if bilingualDialogue(dialogue) {
		return nil, fmt.Errorf("对白包含双语版本，请明确实际发音台词，将翻译放入单独的翻译列")
	}
	lines := []string{}
	for _, part := range htmlBreak.Split(dialogue, -1) {
		if part = strings.TrimSpace(part); part != "" && part != "-" && part != "无" && part != "—" {
			lines = append(lines, part)
		}
	}
	if len(lines) == 0 && narration != "" && narration != "-" && narration != "无" {
		shot.Speaker, shot.Dialogue = "旁白", trimQuotes(narration)
		return []ScriptShot{shot}, nil
	}
	if len(lines) == 0 {
		return []ScriptShot{shot}, nil
	}
	result := make([]ScriptShot, 0, len(lines))
	for index, line := range lines {
		item := shot
		item.Characters = append([]string(nil), shot.Characters...)
		item.Props = append([]string(nil), shot.Props...)
		if match := importedSpeakerPrefix.FindStringSubmatch(line); match != nil && !isStoryboardMetaKey(match[1]) {
			item.Speaker, line = match[1], match[2]
		} else if item.Speaker == "" && len(item.Characters) == 1 {
			item.Speaker = item.Characters[0]
		}
		// “（冷笑）等我做什么”里的括号是表演提示，不是要说出口的台词，移到动作里。
		if cue := leadingCue.FindString(line); cue != "" {
			item.Action = joinField(item.Action, (item.Speaker + cue))
			line = strings.TrimSpace(line[len(cue):])
		}
		item.Dialogue = trimQuotes(line)
		if len(lines) > 1 {
			item.DurationSeconds = shot.DurationSeconds / float64(len(lines))
			if index > 0 {
				item.Transition = ""
			}
		}
		result = append(result, item)
	}
	return result, nil
}

var importedSpeakerPrefix = regexp.MustCompile(`^\s*([^：:“”"「」（）()\s，。！？,.!?·]{1,30})\s*(?:（[^）]*）|\([^)]*\))?\s*[：:·]\s*(.+)$`)
var soundNote = regexp.MustCompile(`^(?:无(?:可见)?(?:字幕|台词|对白)|(?:动作|环境|配乐|音乐|光效|声效|音效|脚步|呼吸|字幕|声音|拟音).*(?:声|音|待|卡点|尾部|开端)|[^：:·]*(?:字幕|字卡)(?:尾部|开端)$|(?:待听校|待核|静音))`)
var hanText = regexp.MustCompile(`[\p{Han}]`)
var latinText = regexp.MustCompile(`[A-Za-z]`)

func bilingualDialogue(text string) bool {
	for _, line := range htmlBreak.Split(text, -1) {
		if match := importedSpeakerPrefix.FindStringSubmatch(line); match != nil {
			line = match[2]
		}
		for _, sep := range []string{"／", "/", "中文翻译", "英文翻译"} {
			left, right, ok := strings.Cut(line, sep)
			if ok && ((latinText.MatchString(left) && !hanText.MatchString(left) && hanText.MatchString(right)) || (hanText.MatchString(left) && latinText.MatchString(right) && !hanText.MatchString(right))) {
				return true
			}
		}
	}
	return false
}

func storyboardSoundCell(cell string) (string, string, error) {
	var dialogue, notes []string
	for _, part := range htmlBreak.Split(cell, -1) {
		part = strings.TrimSpace(part)
		if part == "" || part == "无" || part == "—" || part == "-" {
			continue
		}
		if soundNote.MatchString(part) {
			notes = append(notes, "声音备注："+part)
		} else if importedSpeakerPrefix.MatchString(part) {
			dialogue = append(dialogue, part)
		} else {
			return "", "", fmt.Errorf("“对白与声音”内容无法区分台词和备注，请用“说话人：台词”，或拆为对白、声音备注两列")
		}
	}
	return strings.Join(dialogue, "<br>"), strings.Join(notes, "；"), nil
}

func validateImportedSeconds(seconds float64) error {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || (seconds > 0 && seconds < 0.001) || seconds > MaxStoryboardShotSeconds {
		return fmt.Errorf("明确的镜头时长需在 0.001–%d 秒之间（未填写可留空）", MaxStoryboardShotSeconds)
	}
	return nil
}

func joinField(current, value string) string {
	if current == "" {
		return value
	}
	return current + "；" + value
}

func splitNames(value string) []string {
	names := []string{}
	for _, name := range strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune("、,，/／;；", r) }) {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func trimQuotes(value string) string {
	value = strings.TrimSpace(value)
	for _, pair := range [][2]string{{"“", "”"}, {"「", "」"}, {"『", "』"}, {`"`, `"`}} {
		if strings.HasPrefix(value, pair[0]) && strings.HasSuffix(value, pair[1]) && len(value) > len(pair[0])+len(pair[1]) {
			return strings.TrimSpace(value[len(pair[0]) : len(value)-len(pair[1])])
		}
	}
	return value
}
