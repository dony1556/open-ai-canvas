package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"yingce/backend/internal/model"
)

type AdminArkAssetQuery struct {
	Name      string `form:"name"`
	GroupID   string `form:"groupId"`
	GroupType string `form:"groupType"`
	Status    string `form:"status"`
	NextToken string `form:"nextToken"`
}

type AdminArkAsset struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	URL               string `json:"url"`
	GroupID           string `json:"groupId"`
	AssetType         string `json:"assetType"`
	Status            string `json:"status"`
	ProjectName       string `json:"projectName"`
	CreateTime        string `json:"createTime"`
	LastInferenceTime string `json:"lastInferenceTime"`
	Error             struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type AdminArkAssetPage struct {
	Items     []AdminArkAsset `json:"items"`
	NextToken string          `json:"nextToken"`
}

type AdminArkAssetRenameRequest struct {
	Name string `json:"name"`
}

func validateArkPrivateAssetCredentials(setting arkPrivateAssetSettingValue) error {
	if setting.Region == "" || setting.ProjectName == "" || setting.AccessKeyID == "" || setting.AccessKeySecret == "" {
		return BadAuthRequest("方舟素材库尚未配置完整，请由管理员填写 Region、ProjectName 和 IAM AK/SK")
	}
	return nil
}

func (s *Service) adminArkAssetSetting(actor *model.User) (arkPrivateAssetSettingValue, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return arkPrivateAssetSettingValue{}, err
	}
	_, setting, err := s.readArkPrivateAssetSetting()
	if err != nil {
		return setting, err
	}
	return setting, validateArkPrivateAssetCredentials(setting)
}

func arkAssetListPayload(query AdminArkAssetQuery, projectName string) (map[string]interface{}, error) {
	groupType := defaultString(strings.TrimSpace(query.GroupType), "AIGC")
	if groupType != "AIGC" && groupType != "LivenessFace" {
		return nil, BadAuthRequest("素材组类型无效")
	}
	if query.Status != "" && query.Status != "Active" && query.Status != "Processing" && query.Status != "Failed" {
		return nil, BadAuthRequest("素材状态无效")
	}
	if utf8.RuneCountInString(query.Name) > 64 || len(query.GroupID) > 160 || len(query.NextToken) > 8192 {
		return nil, BadAuthRequest("素材筛选参数过长")
	}
	filter := map[string]interface{}{"GroupType": groupType}
	if name := strings.TrimSpace(query.Name); name != "" {
		filter["Name"] = name
	}
	if id := strings.TrimSpace(query.GroupID); id != "" {
		filter["GroupIds"] = []string{id}
	}
	if query.Status != "" {
		filter["Statuses"] = []string{query.Status}
	}
	payload := map[string]interface{}{"Filter": filter, "ProjectName": projectName, "MaxResults": 20, "SortBy": "CreateTime", "SortOrder": "Desc"}
	if query.NextToken != "" {
		payload["NextToken"] = query.NextToken
	}
	return payload, nil
}

// 方舟成功信封必须包含 Result；不能把异常响应当作空列表。
func decodeArkAssetResult(response map[string]interface{}, target interface{}) error {
	result, ok := response["Result"].(map[string]interface{})
	if !ok {
		return errors.New("方舟素材库返回格式无效：缺少 Result")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func (s *Service) AdminArkAssets(ctx context.Context, actor *model.User, query AdminArkAssetQuery) (*AdminArkAssetPage, error) {
	setting, err := s.adminArkAssetSetting(actor)
	if err != nil {
		return nil, err
	}
	payload, err := arkAssetListPayload(query, setting.ProjectName)
	if err != nil {
		return nil, err
	}
	response, err := callArkPrivateAssetAPI(ctx, setting, "ListAssets", payload)
	if err != nil {
		return nil, err
	}
	page := &AdminArkAssetPage{Items: []AdminArkAsset{}}
	result, ok := response["Result"].(map[string]interface{})
	if !ok || result["Items"] == nil {
		return nil, errors.New("方舟素材列表返回格式无效：缺少 Items")
	}
	if err := decodeArkAssetResult(response, page); err != nil {
		return nil, err
	}
	if page.Items == nil {
		page.Items = []AdminArkAsset{}
	}
	for _, item := range page.Items {
		if item.ID == "" {
			return nil, errors.New("方舟素材列表返回了缺少 ID 的素材")
		}
	}
	return page, nil
}

func getAdminArkAsset(ctx context.Context, setting arkPrivateAssetSettingValue, id string) (*AdminArkAsset, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 160 {
		return nil, BadAuthRequest("方舟素材 ID 无效")
	}
	response, err := callArkPrivateAssetAPI(ctx, setting, "GetAsset", map[string]interface{}{"Id": id, "ProjectName": setting.ProjectName})
	if err != nil {
		return nil, err
	}
	var asset AdminArkAsset
	if err := decodeArkAssetResult(response, &asset); err != nil {
		return nil, err
	}
	if asset.ID != id || (asset.ProjectName != "" && asset.ProjectName != setting.ProjectName) {
		return nil, errors.New("方舟返回的素材与当前项目不匹配")
	}
	return &asset, nil
}

func (s *Service) RenameAdminArkAsset(ctx context.Context, actor *model.User, id string, req AdminArkAssetRenameRequest) error {
	setting, err := s.adminArkAssetSetting(actor)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return BadAuthRequest("素材名称须为 1–64 个字符")
	}
	if _, err := getAdminArkAsset(ctx, setting, id); err != nil {
		return err
	}
	if _, err := callArkPrivateAssetAPI(ctx, setting, "UpdateAsset", map[string]interface{}{"Id": strings.TrimSpace(id), "ProjectName": setting.ProjectName, "Name": name}); err != nil {
		return err
	}
	if err := s.repo.RenameArkPortraitAssetsForArkAsset(strings.TrimSpace(id), name); err != nil {
		return fmt.Errorf("方舟端已改名，但本地人像库名称更新失败，请管理员排查：%w", err)
	}
	return nil
}

func (s *Service) DeleteAdminArkAsset(ctx context.Context, actor *model.User, id string) error {
	setting, err := s.adminArkAssetSetting(actor)
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if _, err := getAdminArkAsset(ctx, setting, id); err != nil {
		return err
	}
	bindings, err := s.repo.ArkPrivateAssetBindingsForAsset(id, setting.ProjectName)
	if err != nil {
		return err
	}
	if source, err := s.repo.ArkAssetPersistentReference(id); err != nil {
		return err
	} else if source != "" {
		return BadAuthRequest("该方舟素材仍被引用，请先解除引用：" + source)
	}
	// 活跃任务的输入可能加密，在 service 解密后检查，不能对密文做 LIKE 查询。
	tasks, err := s.repo.ActiveTasksForArkAssetDeletion()
	if err != nil {
		return err
	}
	resourceIDs := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		resourceIDs[binding.ResourceID] = struct{}{}
	}
	for _, task := range tasks {
		input, err := s.decryptTaskInputJSON(task.InputJSON)
		if err != nil {
			return fmt.Errorf("检查任务引用失败，已停止删除：%w", err)
		}
		used := strings.Contains(input, `"`+id+`"`) || strings.Contains(input, "asset://"+id+`"`)
		used = used || documentReferencesResources(input, resourceIDs)
		if used {
			return BadAuthRequest("该素材被排队或运行中的任务引用，请等待任务结束后删除：" + task.ID)
		}
	}
	if _, err := callArkPrivateAssetAPI(ctx, setting, "DeleteAsset", map[string]interface{}{"Id": id, "ProjectName": setting.ProjectName}); err != nil {
		return err
	}
	if err := s.repo.MarkArkPrivateAssetDeleted(id, setting.ProjectName); err != nil {
		return fmt.Errorf("方舟端已删除，但本地绑定更新失败，请管理员排查：%w", err)
	}
	return nil
}
