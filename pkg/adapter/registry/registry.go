package adapterregistry

import (
	"fmt"

	"github.com/silestar/AIGateway/pkg/adapter"
	"github.com/silestar/AIGateway/pkg/adapter/anthropic"
	"github.com/silestar/AIGateway/pkg/adapter/gemini"
	"github.com/silestar/AIGateway/pkg/adapter/openai"
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
	// extraTestEndpoints 插件额外注册的测试端点
	extraTestEndpoints = map[string][]adapter.TestEndpointInfo{}
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

// GetChannelTestEndpoints 获取某渠道类型的所有测试端点（内置 + 插件注册的）
func GetChannelTestEndpoints(channelType string) []adapter.TestEndpointInfo {
	result := make([]adapter.TestEndpointInfo, 0)

	// 1. 从适配器获取内置端点
	a, err := GetAdapter(channelType)
	if err == nil {
		if provider, ok := a.(adapter.TestEndpointProvider); ok {
			result = append(result, provider.TestEndpoints()...)
		}
	}

	// openai-response 与 openai 共享端点
	if channelType == "openai-response" {
		a2, err2 := GetAdapter("openai")
		if err2 == nil {
			if provider, ok := a2.(adapter.TestEndpointProvider); ok {
				result = append(result, provider.TestEndpoints()...)
			}
		}
	}

	// 如果没有任何端点，提供默认的自动检测
	if len(result) == 0 {
		result = append(result, adapter.TestEndpointInfo{
			ID: "auto", Label: "自动检测（默认）", IsAuto: true,
		})
	}

	// 2. 追加插件额外注册的端点
	if extras, ok := extraTestEndpoints[channelType]; ok {
		result = append(result, extras...)
	}

	return result
}

// RegisterTestEndpoint 插件注册测试端点
func RegisterTestEndpoint(channelType string, info adapter.TestEndpointInfo) {
	extraTestEndpoints[channelType] = append(extraTestEndpoints[channelType], info)
}
