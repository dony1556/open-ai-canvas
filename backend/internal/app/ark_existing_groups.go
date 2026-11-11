package app

import (
	"context"
	"errors"
	"yingce/backend/internal/model"
)

type ArkExistingGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	GroupType string `json:"groupType"`
}

type ArkExistingGroupPage struct {
	Items     []ArkExistingGroup `json:"items"`
	NextToken string             `json:"nextToken"`
}

func (s *Service) ArkExistingGroups(ctx context.Context, actor *model.User, query AdminArkAssetQuery) (*ArkExistingGroupPage, error) {
	setting, err := s.arkExistingLibrarySetting(actor)
	if err != nil {
		return nil, err
	}
	payload, err := arkAssetListPayload(query, setting.ProjectName)
	if err != nil {
		return nil, err
	}
	payload["MaxResults"] = 100
	response, err := callArkPrivateAssetAPI(ctx, setting, "ListAssetGroups", payload)
	if err != nil {
		return nil, err
	}
	result, ok := response["Result"].(map[string]interface{})
	if !ok || result["Items"] == nil {
		return nil, errors.New("方舟素材组列表格式无效：缺少 Items")
	}
	page := &ArkExistingGroupPage{}
	if err := decodeArkAssetResult(response, page); err != nil {
		return nil, err
	}
	if page.Items == nil {
		page.Items = []ArkExistingGroup{}
	}
	for _, group := range page.Items {
		if group.ID == "" {
			return nil, errors.New("方舟返回了缺少 ID 的素材组")
		}
	}
	return page, nil
}
