package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/middleware"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/response"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	service ports.UserService
	statsd  statsd.ClientInterface
}

func NewUserHandler(service ports.UserService, statsdClient statsd.ClientInterface) *UserHandler {
	return &UserHandler{service: service, statsd: statsdClient}
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.RespondError(c, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid user id")
		return
	}

	role, _ := c.Get(middleware.ContextUserRole)
	userID, _ := c.Get(middleware.ContextUserID)

	isAdmin := domain.Role(role.(string)) == domain.RoleAdministrator
	isOwner := userID.(string) == id.String()

	if !isAdmin && !isOwner {
		response.RespondError(c, http.StatusForbidden, "FORBIDDEN", "access denied")
		return
	}

	resp, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			response.RespondError(c, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.RespondError(c, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid user id")
		return
	}

	var req ports.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondError(c, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error())
		return
	}

	resp, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			_ = h.statsd.Count("video_processor.users.updated", 1, []string{"result:failure", "reason:not_found"}, 1)
			response.RespondError(c, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
			return
		}
		_ = h.statsd.Count("video_processor.users.updated", 1, []string{"result:failure", "reason:invalid_payload"}, 1)
		response.RespondError(c, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error())
		return
	}

	_ = h.statsd.Count("video_processor.users.updated", 1, []string{"result:success"}, 1)
	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.RespondError(c, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid user id")
		return
	}

	resp, err := h.service.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			_ = h.statsd.Count("video_processor.users.deleted", 1, []string{"result:failure", "reason:not_found"}, 1)
			response.RespondError(c, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
			return
		}
		_ = h.statsd.Count("video_processor.users.deleted", 1, []string{"result:failure", "reason:internal_error"}, 1)
		response.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	_ = h.statsd.Count("video_processor.users.deleted", 1, []string{"result:success"}, 1)
	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) List(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.service.List(c.Request.Context(), limit, offset)
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}
