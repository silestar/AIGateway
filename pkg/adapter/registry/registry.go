package adapterregistry

import (
	"fmt"

	"github.com/bokelife/aigateway/pkg/adapter"
	"github.com/bokelife/aigateway/pkg/adapter/anthropic"
	"github.com/bokelife/aigateway/pkg/adapter/gemini"
	"github.com/bokelife/aigateway/pkg/adapter/openai"
)

var (
	ErrUnsupportedChannelType = fmt.Errorf("unsupported channel type")
	adapters                  = map[string]adapter.ChannelAdapter{
		"openai":    &openai.Adapter{},
		"anthropic": &anthropic.Adapter{},
		"gemini":    &gemini.Adapter{},
	}
)

// RegisterAdapter 注册适配器
func RegisterAdapter(channelType string, a adapter.ChannelAdapter) {
	adapters[channelType] = a
}

// GetAdapter 根据渠道类型获取适配器
func GetAdapter(channelType string) (adapter.ChannelAdapter, error) {
	a, ok := adapters[channelType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChannelType, channelType)
	}
	return a, nil
}
