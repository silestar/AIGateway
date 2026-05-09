package adapterregistry

import (
	"fmt"

	"github.com/bokelife/aigateway/pkg/adapter"
	"github.com/bokelife/aigateway/pkg/adapter/anthropic"
	"github.com/bokelife/aigateway/pkg/adapter/gemini"
	"github.com/bokelife/aigateway/pkg/adapter/openai"
)

// ChannelTypeInfo 渠道类型信息（供 API 返回）
type ChannelTypeInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	IsPlugin    bool   `json:"is_plugin"`
	BaseURL     string `json:"base_url,omitempty"`
	Description string `json:"description,omitempty"`
}

var (
	ErrUnsupportedChannelType = fmt.Errorf("unsupported channel type")
	adapters                  = map[string]adapter.ChannelAdapter{}
	// channelTypeMeta 渠道类型元数据（内置 + 插件注册的）
	channelTypeMeta = map[string]ChannelTypeInfo{
		"openai":    {Type: "openai", Name: "OpenAI", IsPlugin: false},
		"anthropic": {Type: "anthropic", Name: "Anthropic", IsPlugin: false},
		"gemini":    {Type: "gemini", Name: "Google Gemini", IsPlugin: false},
	}
)

func init() {
	// 注册内置适配器
	adapters["openai"] = &openai.Adapter{}
	adapters["anthropic"] = &anthropic.Adapter{}
	adapters["gemini"] = &gemini.Adapter{}
}

// RegisterAdapter 注册适配器
func RegisterAdapter(channelType string, a adapter.ChannelAdapter) {
	adapters[channelType] = a
}

// RegisterChannelType 注册渠道类型元数据（插件使用）
func RegisterChannelType(info ChannelTypeInfo) {
	channelTypeMeta[info.Type] = info
}

// GetAdapter 根据渠道类型获取适配器
func GetAdapter(channelType string) (adapter.ChannelAdapter, error) {
	a, ok := adapters[channelType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChannelType, channelType)
	}
	return a, nil
}

// ListChannelTypes 列出所有渠道类型信息
func ListChannelTypes() []ChannelTypeInfo {
	result := make([]ChannelTypeInfo, 0, len(channelTypeMeta))
	for _, info := range channelTypeMeta {
		result = append(result, info)
	}
	return result
}
