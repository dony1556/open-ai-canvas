package app

import (
	"encoding/json"
	"fmt"
)

// Resolve declared limits only. A logical model must fit every possible text route.
func (s *Service) dramaTextContextLimits(choice DramaModelChoice) (int, int) {
	if choice.LogicalModelID == "" {
		cap := s.cloudAgentChannelTextCapability(CloudAgentRequest{ChannelID: choice.ChannelID, ChannelModelKey: choice.ModelKey})
		if cap != nil {
			return cap.ContextWindowTokens, cap.MaxOutputTokens
		}
		return 0, 0
	}
	snapshot, err := s.routeCatalogSnapshot()
	if err != nil || snapshot == nil {
		return 0, 0
	}
	cached, ok := snapshot.Models[choice.LogicalModelID]
	if !ok {
		return 0, 0
	}
	window, output := 0, 0
	for _, route := range cached.Routes {
		if normalizeCapability(route.CapabilitySpec.Capability) != "text" {
			continue
		}
		cap, err := normalizedChannelModelCapability(&route.ChannelModel)
		if err != nil || cap == nil || cap.Text == nil || cap.Text.ContextWindowTokens <= 0 || cap.Text.MaxOutputTokens <= 0 {
			return 0, 0
		}
		if window == 0 || cap.Text.ContextWindowTokens < window {
			window = cap.Text.ContextWindowTokens
		}
		if output == 0 || cap.Text.MaxOutputTokens < output {
			output = cap.Text.MaxOutputTokens
		}
	}
	return window, output
}

func dramaTextContextReport(input map[string]any, window, modelOutput int) (string, error) {
	options, _ := input["textOptions"].(map[string]any)
	output := intFromAny(options["maxOutputTokens"])
	if output <= 0 {
		output = modelOutput
	}
	if output <= 0 {
		output = 4096
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("无法估算文本请求上下文：%w", err)
	}
	tokens := cloudAgentEstimatedTokens(encoded)
	if window <= 0 {
		return fmt.Sprintf("模型上下文容量未知；输入保守估算 %d tokens，输出预留 %d tokens，无法确认完整容纳。正文完整发送，由上游最终校验。", tokens, output), nil
	}
	if modelOutput > 0 && output > modelOutput {
		return "", fmt.Errorf("输出预留 %d tokens 超过模型输出上限 %d tokens", output, modelOutput)
	}
	overhead := max(512, min(8192, window/50))
	if tokens+output+overhead > window {
		return "", fmt.Errorf("文本请求超过模型上下文预算：输入保守估算 %d tokens（含系统提示词、剧本、技能与引用），输出预留 %d，封装预留 %d，模型窗口 %d。请选择技能章节、减少引用或缩短原文；不会截断发送", tokens, output, overhead, window)
	}
	// Bind the actual output limit to the reservation used by this admission check.
	if options == nil {
		options = map[string]any{}
	}
	options["maxOutputTokens"] = output
	input["textOptions"] = options
	return fmt.Sprintf("输入保守估算 %d + 输出预留 %d + 封装预留 %d / 模型窗口 %d tokens；完整发送，估算不等于精确分词。", tokens, output, overhead, window), nil
}
