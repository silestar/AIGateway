package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"

	adapterregistry "github.com/bokelife/aigateway/pkg/adapter/registry"
)

type service struct {
	db *gorm.DB
}

// NewService 创建渠道服务
func NewService(db *gorm.DB) ChannelService {
	return &service{db: db}
}

func (s *service) Create(ctx context.Context, name, channelType, baseURL string) (*Channel, error) {
	// 校验渠道类型
	if _, err := adapterregistry.GetAdapter(channelType); err != nil {
		return nil, fmt.Errorf("unsupported channel type: %s", channelType)
	}

	ch := &Channel{
		Name:    name,
		Type:    channelType,
		BaseURL: baseURL,
		Status:  "active",
		Weight:  0,
	}

	if err := s.db.WithContext(ctx).Create(ch).Error; err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return ch, nil
}

func (s *service) GetById(ctx context.Context, id uint) (*Channel, error) {
	var ch Channel
	if err := s.db.WithContext(ctx).First(&ch, id).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *service) List(ctx context.Context, filter ListFilter) ([]ChannelListItem, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	var total int64
	query := s.db.WithContext(ctx).Model(&Channel{})

	// 状态筛选
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	// 类型筛选
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	// 搜索：按名称/ID/类型模糊匹配，或按模型名称匹配
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		// 子查询：找到包含匹配模型的渠道ID
		modelSubQuery := s.db.Model(&ChannelModel{}).
			Select("DISTINCT channel_id").
			Where("display_model_name LIKE ? OR actual_model_name LIKE ?", searchPattern, searchPattern)

		query = query.Where(
			"name LIKE ? OR CAST(id AS TEXT) LIKE ? OR type LIKE ? OR id IN (?)",
			searchPattern, searchPattern, searchPattern, modelSubQuery,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	sortBy := "id"
	sortOrder := "DESC"
	switch filter.SortBy {
	case "weight":
		sortBy = "weight"
	case "latency":
		sortBy = "last_test_latency"
	case "id":
		sortBy = "id"
	}
	if filter.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	var channels []Channel
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder)).Offset(offset).Limit(filter.PageSize).Find(&channels).Error; err != nil {
		return nil, 0, err
	}

	// 组装列表项，附带账号统计和分组信息
	items := make([]ChannelListItem, 0, len(channels))
	for _, ch := range channels {
		item := ChannelListItem{Channel: ch}

		// 账号统计
		var totalAcc, activeAcc int64
		s.db.WithContext(ctx).Table("channel_accounts").
			Where("channel_id = ?", ch.ID).
			Count(&totalAcc)
		s.db.WithContext(ctx).Table("channel_accounts").
			Where("channel_id = ? AND status = ?", ch.ID, "active").
			Count(&activeAcc)
		item.TotalAccountCount = int(totalAcc)
		item.ActiveAccountCount = int(activeAcc)

		// 分组信息
		var groups []GroupInfo
		s.db.WithContext(ctx).Raw(
			"SELECT cg.id, cg.name FROM channel_groups cg "+
				"JOIN channel_group_members cgm ON cg.id = cgm.group_id "+
				"WHERE cgm.channel_id = ?", ch.ID,
		).Scan(&groups)
		item.Groups = groups

		items = append(items, item)
	}

	return items, total, nil
}

func (s *service) Update(ctx context.Context, id uint, name, baseURL string, weight, maxRPM, maxTPM, maxDailyRequests int) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "base_url": baseURL, "weight": weight, "max_rpm": maxRPM, "max_tpm": maxTPM, "max_daily_requests": maxDailyRequests}).Error
}

func (s *service) UpdateStatus(ctx context.Context, id uint, status string) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Update("status", status).Error
}

func (s *service) UpdateWeight(ctx context.Context, id uint, weight int) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Update("weight", weight).Error
}

func (s *service) TestConnection(ctx context.Context, channelType, baseURL, apiKey string) error {
	adp, err := adapterregistry.GetAdapter(channelType)
	if err != nil {
		return fmt.Errorf("unsupported channel type: %s", channelType)
	}
	_, err = adp.FetchModels(ctx, baseURL, apiKey)
	return err
}

func (s *service) Delete(ctx context.Context, id uint) error {
	tx := s.db.WithContext(ctx).Begin()
	// 删除关联的模型映射
	if err := tx.Where("channel_id = ?", id).Delete(&ChannelModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	// 删除关联的账号
	if err := tx.Exec("DELETE FROM channel_accounts WHERE channel_id = ?", id).Error; err != nil {
		tx.Rollback()
		return err
	}
	// 删除关联的分组成员
	if err := tx.Exec("DELETE FROM channel_group_members WHERE channel_id = ?", id).Error; err != nil {
		tx.Rollback()
		return err
	}
	// 删除渠道
	if err := tx.Delete(&Channel{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *service) FetchModels(ctx context.Context, id uint, testKey string) ([]ModelInfo, error) {
	var ch Channel
	if err := s.db.WithContext(ctx).First(&ch, id).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 获取适配器
	adp, err := adapterregistry.GetAdapter(ch.Type)
	if err != nil {
		return nil, err
	}

	adapterModels, err := adp.FetchModels(ctx, ch.BaseURL, testKey)
	if err != nil {
		return nil, err
	}
	// 转换 adapter.ModelInfo → channel.ModelInfo
	result := make([]ModelInfo, len(adapterModels))
	for i, m := range adapterModels {
		result[i] = ModelInfo{ID: m.ID, OwnedBy: m.OwnedBy}
	}
	return result, nil
}

func (s *service) GetModelsByChannel(ctx context.Context, id uint) ([]ChannelModel, error) {
	var models []ChannelModel
	if err := s.db.WithContext(ctx).Where("channel_id = ?", id).Order("display_model_name").Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

func (s *service) SaveModels(ctx context.Context, id uint, models []ChannelModel) error {
	tx := s.db.WithContext(ctx).Begin()

	// 删除旧的模型映射
	if err := tx.Where("channel_id = ?", id).Delete(&ChannelModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 批量插入新的模型映射
	for i := range models {
		models[i].ChannelID = id
		if err := tx.Create(&models[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// TestChannel 测试渠道可用性（发送一次轻量请求）
func (s *service) TestChannel(ctx context.Context, id uint, apiKey string) (*TestResult, error) {
	var ch Channel
	if err := s.db.WithContext(ctx).First(&ch, id).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 确定测试模型
	testModel := ch.TestModel
	if testModel == "" {
		// 取第一个已配置的模型
		var cm ChannelModel
		if err := s.db.WithContext(ctx).Where("channel_id = ? AND status = ?", id, "enabled").Order("display_model_name").First(&cm).Error; err != nil {
			return nil, fmt.Errorf("该渠道没有已配置的模型，请先配置模型或指定测试模型")
		}
		testModel = cm.ActualModelName
		if testModel == "" {
			testModel = cm.DisplayModelName
		}
	}

	// 构建轻量请求
	reqBody := map[string]interface{}{
		"model":       testModel,
		"messages":    []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens":  5,
		"stream":      false,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := ch.BaseURL + "/v1/chat/completions"
	if ch.Type == "anthropic" {
		url = ch.BaseURL + "/v1/messages"
		reqBody = map[string]interface{}{
			"model":      testModel,
			"messages":   []map[string]string{{"role": "user", "content": "hi"}},
			"max_tokens": 5,
		}
		bodyBytes, _ = json.Marshal(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置请求头
	if ch.Type == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	result := &TestResult{
		Model:   testModel,
		Latency: latency,
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			result.Success = false
			result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		} else {
			result.Success = true
		}
	}

	// 更新渠道测试记录
	now := time.Now()
	s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_test_latency": latency,
		"last_tested_at":   now,
	})

	return result, nil
}

// BatchTestModels 批量测试指定模型
func (s *service) BatchTestModels(ctx context.Context, id uint, modelNames []string, apiKey string) ([]BatchTestResultItem, error) {
	var ch Channel
	if err := s.db.WithContext(ctx).First(&ch, id).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	results := make([]BatchTestResultItem, 0, len(modelNames))
	for _, modelName := range modelNames {
		result := BatchTestResultItem{Model: modelName}

		reqBody := map[string]interface{}{
			"model":      modelName,
			"messages":   []map[string]string{{"role": "user", "content": "hi"}},
			"max_tokens": 5,
			"stream":     false,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		url := ch.BaseURL + "/v1/chat/completions"
		if ch.Type == "anthropic" {
			url = ch.BaseURL + "/v1/messages"
			reqBody = map[string]interface{}{
				"model":      modelName,
				"messages":   []map[string]string{{"role": "user", "content": "hi"}},
				"max_tokens": 5,
			}
			bodyBytes, _ = json.Marshal(reqBody)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		if ch.Type == "anthropic" {
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Content-Type", "application/json")
		}

		start := time.Now()
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			result.Latency = int(time.Since(start).Milliseconds())

			if err != nil {
				result.Success = false
				result.Error = err.Error()
			} else {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				result.Status = resp.StatusCode
				if resp.StatusCode >= 400 {
					result.Success = false
					result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
				} else {
					result.Success = true
				}
			}

		results = append(results, result)
	}

	// 更新渠道测试记录
	now := time.Now()
	s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_tested_at": now,
	})

	return results, nil
}

// UpdateTestModel 更新渠道指定测试模型
func (s *service) UpdateTestModel(ctx context.Context, id uint, testModel string) error {
	return s.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).
		Update("test_model", testModel).Error
}

// CopyChannel 复制渠道（基本信息+模型映射，不含账号，新渠道默认禁用）
func (s *service) CopyChannel(ctx context.Context, id uint) (*Channel, error) {
	var src Channel
	if err := s.db.WithContext(ctx).First(&src, id).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 复制基本信息
	newCh := &Channel{
		Name:       src.Name + " - 复制",
		Type:       src.Type,
		BaseURL:    src.BaseURL,
		Status:     "disabled", // 默认禁用
		Weight:     src.Weight,
		MaxRPM:     src.MaxRPM,
		MaxTPM:     src.MaxTPM,
		MaxDailyRequests: src.MaxDailyRequests,
		TestModel: src.TestModel,
	}

	tx := s.db.WithContext(ctx).Begin()

	if err := tx.Create(newCh).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("create copy: %w", err)
	}

	// 复制模型映射
	var models []ChannelModel
	if err := tx.Where("channel_id = ?", id).Find(&models).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	for i := range models {
		models[i].ID = 0
		models[i].ChannelID = newCh.ID
		if err := tx.Create(&models[i]).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return newCh, nil
}
