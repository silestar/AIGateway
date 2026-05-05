package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bokelife/aigateway/internal/group"
)

// GroupHandler 分组管理 API
type GroupHandler struct {
	router *group.Router
}

func NewGroupHandler(router *group.Router) *GroupHandler {
	return &GroupHandler{router: router}
}

// RegisterRoutes 注册分组路由
func (h *GroupHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// 渠道分组
	channelGroups := rg.Group("/channel-groups")
	channelGroups.GET("", h.ListChannelGroups)
	channelGroups.POST("", h.CreateChannelGroup)
	channelGroups.PUT("/:id", h.UpdateChannelGroup)
	channelGroups.DELETE("/:id", h.DeleteChannelGroup)
	channelGroups.POST("/:id/channels", h.AddChannelToGroup)
	channelGroups.DELETE("/:id/channels/:channel_id", h.RemoveChannelFromGroup)

	// 消费者分组
	consumerGroups := rg.Group("/consumer-groups")
	consumerGroups.GET("", h.ListConsumerGroups)
	consumerGroups.POST("", h.CreateConsumerGroup)
	consumerGroups.PUT("/:id", h.UpdateConsumerGroup)
	consumerGroups.DELETE("/:id", h.DeleteConsumerGroup)
	consumerGroups.POST("/:id/consumers", h.AddConsumerToGroup)
	consumerGroups.DELETE("/:id/consumers/:consumer_id", h.RemoveConsumerFromGroup)

	// 绑定关系
	bindings := rg.Group("/group-bindings")
	bindings.POST("", h.BindChannelGroup)
	bindings.DELETE("/:consumer_group_id/:channel_group_id", h.UnbindChannelGroup)
}

// ========== 渠道分组 ==========

func (h *GroupHandler) CreateChannelGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Weight      int    `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	cg, err := h.router.CreateChannelGroup(c.Request.Context(), req.Name, req.Description, req.Weight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cg})
}

func (h *GroupHandler) ListChannelGroups(c *gin.Context) {
	groups, err := h.router.ListChannelGroups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": groups, "total": len(groups)})
}

func (h *GroupHandler) UpdateChannelGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Weight      int    `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.UpdateChannelGroup(c.Request.Context(), id, req.Name, req.Description, req.Weight); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *GroupHandler) DeleteChannelGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	if err := h.router.DeleteChannelGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *GroupHandler) AddChannelToGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		ChannelID uint `json:"channel_id" binding:"required"`
		Weight    int  `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.AddChannelToGroup(c.Request.Context(), id, req.ChannelID, req.Weight); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": id, "channel_id": req.ChannelID}})
}

func (h *GroupHandler) RemoveChannelFromGroup(c *gin.Context) {
	groupID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_group_id", err.Error()))
		return
	}
	channelID, err := parseIDFromParam(c, "channel_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_id", err.Error()))
		return
	}

	if err := h.router.RemoveChannelFromGroup(c.Request.Context(), groupID, channelID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": groupID, "channel_id": channelID}})
}

// ========== 消费者分组 ==========

func (h *GroupHandler) CreateConsumerGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	cg, err := h.router.CreateConsumerGroup(c.Request.Context(), req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cg})
}

func (h *GroupHandler) ListConsumerGroups(c *gin.Context) {
	groups, err := h.router.ListConsumerGroups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": groups, "total": len(groups)})
}

func (h *GroupHandler) UpdateConsumerGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.UpdateConsumerGroup(c.Request.Context(), id, req.Name, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *GroupHandler) DeleteConsumerGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	if err := h.router.DeleteConsumerGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *GroupHandler) AddConsumerToGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		ConsumerID uint `json:"consumer_id" binding:"required"`
		QuotaRPM   int  `json:"quota_rpm"`
		QuotaTPM   int  `json:"quota_tpm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.AddConsumerToGroup(c.Request.Context(), id, req.ConsumerID, req.QuotaRPM, req.QuotaTPM); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": id, "consumer_id": req.ConsumerID}})
}

func (h *GroupHandler) RemoveConsumerFromGroup(c *gin.Context) {
	groupID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_group_id", err.Error()))
		return
	}
	consumerID, err := parseIDFromParam(c, "consumer_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_consumer_id", err.Error()))
		return
	}

	if err := h.router.RemoveConsumerFromGroup(c.Request.Context(), groupID, consumerID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": groupID, "consumer_id": consumerID}})
}

// ========== 绑定 ==========

func (h *GroupHandler) BindChannelGroup(c *gin.Context) {
	var req struct {
		ConsumerGroupID  uint `json:"consumer_group_id" binding:"required"`
		ChannelGroupID   uint `json:"channel_group_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.BindChannelGroup(c.Request.Context(), req.ConsumerGroupID, req.ChannelGroupID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"consumer_group_id": req.ConsumerGroupID, "channel_group_id": req.ChannelGroupID}})
}

func (h *GroupHandler) UnbindChannelGroup(c *gin.Context) {
	consumerGroupID, err := parseIDFromParam(c, "consumer_group_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_consumer_group_id", err.Error()))
		return
	}
	channelGroupID, err := parseIDFromParam(c, "channel_group_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_group_id", err.Error()))
		return
	}

	if err := h.router.UnbindChannelGroup(c.Request.Context(), consumerGroupID, channelGroupID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"consumer_group_id": consumerGroupID, "channel_group_id": channelGroupID}})
}

