package proxy

import (
	"context"
	"net/http"

	"github.com/bokelife/aigateway/internal/account"
	"github.com/bokelife/aigateway/internal/channel"
)

// ProxyEngine 代理引擎接口
type ProxyEngine interface {
	Forward(ctx context.Context, ch *channel.Channel, acc *account.Account, originalReq *http.Request) (*http.Response, error)
}
