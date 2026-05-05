package sdk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// HookHandler 钩子处理函数签名
type HookHandler func(req HookRequest) HookResponse

// HookRequest 钩子请求（与主系统 plugin.HookRequest 一致）
type HookRequest struct {
	ConsumerID        uint                   `json:"consumer_id"`
	ConsumerName      string                 `json:"consumer_name,omitempty"`
	Model             string                 `json:"model"`
	Request           *HookRequestBody       `json:"request,omitempty"`
	Response          *HookResponseBody      `json:"response,omitempty"`
	ChannelID         uint                   `json:"channel_id,omitempty"`
	AccountID         uint                   `json:"account_id,omitempty"`
	CandidateAccounts []CandidateAccount     `json:"candidate_accounts,omitempty"`
	Config            map[string]interface{} `json:"config,omitempty"`
}

type HookRequestBody struct {
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
}

type HookResponseBody struct {
	StatusCode int                    `json:"status_code,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Body       map[string]interface{} `json:"body,omitempty"`
}

type CandidateAccount struct {
	ID       uint   `json:"id"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
}

// HookResponse 钩子响应
type HookResponse struct {
	Action           HookAction             `json:"action"`
	StatusCode       int                    `json:"status_code,omitempty"`
	Message          string                 `json:"message,omitempty"`
	ModifiedRequest  *HookRequestBody       `json:"modified_request,omitempty"`
	ModifiedResponse *HookResponseBody      `json:"modified_response,omitempty"`
	ExcludeIDs       []uint                 `json:"exclude_ids,omitempty"`
}

// HookAction 钩子响应动作
type HookAction string

const (
	ActionContinue    HookAction = "continue"
	ActionReject      HookAction = "reject"
	ActionUseDefault  HookAction = "use_default"
	ActionFilter      HookAction = "filter"
)

var handlers = make(map[string]HookHandler)
var authToken string

// MustGetEnv 获取必需环境变量，若不存在则 panic
func MustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
}

// GetEnv 获取环境变量，不存在返回默认值
func GetEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// HandleHook 注册钩子处理函数
func HandleHook(hookName string, handler HookHandler) {
	handlers[hookName] = handler
}

// Continue 快速构造 continue 响应
func Continue() HookResponse {
	return HookResponse{Action: ActionContinue}
}

// Reject 快速构造 reject 响应
func Reject(statusCode int, message string) HookResponse {
	return HookResponse{Action: ActionReject, StatusCode: statusCode, Message: message}
}

// UseDefault 快速构造 use_default 响应
func UseDefault() HookResponse {
	return HookResponse{Action: ActionUseDefault}
}

// Filter 快速构造 filter 响应
func Filter(excludeIDs []uint) HookResponse {
	return HookResponse{Action: ActionFilter, ExcludeIDs: excludeIDs}
}

// StartPlugin 启动插件 HTTP 服务
func StartPlugin(port string, token string) {
	authToken = token

	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// 钩子端点
	mux.HandleFunc("/hook/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !checkAuth(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// 提取钩子名称：/hook/pre_request → pre_request
		hookName := r.URL.Path[len("/hook/"):]
		handler, ok := handlers[hookName]
		if !ok {
			http.Error(w, "hook not found", http.StatusNotFound)
			return
		}

		var req HookRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		resp := handler(req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// 优雅关闭端点
	mux.HandleFunc("/admin/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		// 异步退出，确保响应已发送
		go func() {
			os.Exit(0)
		}()
	})

	addr := ":" + port
	fmt.Printf("Plugin listening on %s\n", addr)
	if err := http.ListenAndServe("127.0.0.1"+addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Plugin server error: %v\n", err)
		os.Exit(1)
	}
}

func checkAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+authToken
}
