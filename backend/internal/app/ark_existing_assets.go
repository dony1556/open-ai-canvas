package app

import (
	"context"
	"slices"
	"strings"
	"yingce/backend/internal/model"
)

// 全库可见范围独立于素材上传者和素材组授权；写操作仍校验管理员。
func (s *Service) arkExistingLibrarySetting(actor *model.User) (arkPrivateAssetSettingValue, error) {
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		return arkPrivateAssetSettingValue{}, Unauthorized("请先登录")
	}
	_, setting, err := s.readArkPrivateAssetSetting()
	if err != nil {
		return setting, err
	}
	if actor.Role != model.UserRoleAdmin && !setting.LibraryVisibleToAll && !slices.Contains(setting.LibraryVisibleUserIDs, actor.ID) {
		return setting, Forbidden("管理员尚未授权当前账号查看方舟素材库")
	}
	return setting, validateArkPrivateAssetCredentials(setting)
}

func (s *Service) ArkExistingAssets(ctx context.Context, actor *model.User, query AdminArkAssetQuery) (*AdminArkAssetPage, error) {
	setting, err := s.arkExistingLibrarySetting(actor)
	if err != nil {
		return nil, err
	}
	return listExistingArkAssets(ctx, setting, query)
}
