package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/silestar/AIGateway/internal/group"
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
	channelGroups.GET("/:id", h.GetChannelGroup)
	channelGroups.PUT("/:id", h.UpdateChannelGroup)
	channelGroups.DELETE("/:id", h.DeleteChannelGroup)
	channelGroups.POST("/:id/channels", h.AddChannelToGroup)
	channelGroups.PUT("/:id/channels", h.SetChannelGroupChannels)
	channelGroups.DELETE("/:id/channels/:channel_id", h.RemoveChannelFromGroup)

	// 密钥分组
	keysGroups := rg.Group("/keys-groups")
	keysGroups.GET("", h.ListKeysGroups)
	keysGroups.POST("", h.CreateKeysGroup)
	keysGroups.GET("/:id", h.GetKeysGroup)
	keysGroups.PUT("/:id", h.UpdateKeysGroup)
	keysGroups.DELETE("/:id", h.DeleteKeysGroup)
	keysGroups.POST("/:id/keys", h.AddKeysToGroup)
	keysGroups.DELETE("/:id/keys/:keys_id", h.RemoveKeysFromGroup)
	keysGroups.PUT("/:id/channel-groups", h.SetKeysGroupChannelGroups)

	// 绑定关系
	bindings := rg.Group("/group-bindings")
	bindings.POST("", h.BindChannelGroup)
	bindings.DELETE("/:keys_group_id/:channel_group_id", h.UnbindChannelGroup)
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

func (h *GroupHandler) GetChannelGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	detail, err := h.router.GetChannelGroup(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
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

func (h *GroupHandler) SetChannelGroupChannels(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		ChannelIDs []uint `json:"channel_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.SetChannelGroupChannels(c.Request.Context(), id, req.ChannelIDs); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": id}})
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

// ========== 密钥分组 ==========

func (h *GroupHandler) CreateKeysGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		QuotaRPM    int    `json:"quota_rpm"`
		QuotaTPM    int    `json:"quota_tpm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	cg, err := h.router.CreateKeysGroup(c.Request.Context(), req.Name, req.Description, req.QuotaRPM, req.QuotaTPM)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cg})
}

func (h *GroupHandler) ListKeysGroups(c *gin.Context) {
	groups, err := h.router.ListKeysGroups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": groups, "total": len(groups)})
}

func (h *GroupHandler) GetKeysGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	detail, err := h.router.GetKeysGroup(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

func (h *GroupHandler) UpdateKeysGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		QuotaRPM    int    `json:"quota_rpm"`
		QuotaTPM    int    `json:"quota_tpm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.UpdateKeysGroup(c.Request.Context(), id, req.Name, req.Description, req.QuotaRPM, req.QuotaTPM); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *GroupHandler) DeleteKeysGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	if err := h.router.DeleteKeysGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *GroupHandler) AddKeysToGroup(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		KeysID uint `json:"keys_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.AddKeysToGroup(c.Request.Context(), id, req.KeysID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": id, "keys_id": req.KeysID}})
}

func (h *GroupHandler) RemoveKeysFromGroup(c *gin.Context) {
	groupID, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_group_id", err.Error()))
		return
	}
	keysID, err := parseIDFromParam(c, "keys_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_keys_id", err.Error()))
		return
	}

	if err := h.router.RemoveKeysFromGroup(c.Request.Context(), groupID, keysID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": groupID, "keys_id": keysID}})
}

func (h *GroupHandler) SetKeysGroupChannelGroups(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_id", err.Error()))
		return
	}

	var req struct {
		ChannelGroupIDs []uint `json:"channel_group_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.SetKeysGroupChannelGroups(c.Request.Context(), id, req.ChannelGroupIDs); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"group_id": id}})
}

// ========== 绑定 ==========

func (h *GroupHandler) BindChannelGroup(c *gin.Context) {
	var req struct {
		KeysGroupID    uint `json:"keys_group_id" binding:"required"`
		ChannelGroupID uint `json:"channel_group_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_request", err.Error()))
		return
	}

	if err := h.router.BindChannelGroup(c.Request.Context(), req.KeysGroupID, req.ChannelGroupID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"keys_group_id": req.KeysGroupID, "channel_group_id": req.ChannelGroupID}})
}

func (h *GroupHandler) UnbindChannelGroup(c *gin.Context) {
	keysGroupID, err := parseIDFromParam(c, "keys_group_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_keys_group_id", err.Error()))
		return
	}
	channelGroupID, err := parseIDFromParam(c, "channel_group_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid_channel_group_id", err.Error()))
		return
	}

	if err := h.router.UnbindChannelGroup(c.Request.Context(), keysGroupID, channelGroupID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"keys_group_id": keysGroupID, "channel_group_id": channelGroupID}})
}