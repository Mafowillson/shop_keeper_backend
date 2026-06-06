package shop

import (
	"net/http"

	"shop_keeper_backend/internal/api"
	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) getUserID(c *gin.Context) (string, bool) {
	userID, ok := middleware.GetUserID(c)
	return userID, ok
}

func (h *Handler) Create(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	var input CreateShopInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	shop, err := h.service.Create(c.Request.Context(), userID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, shop)
}

func (h *Handler) List(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		api.Unauthorized(c, msgs.Unauthorized)
		return
	}

	page, pageSize, err := api.ParsePagination(c)
	if err != nil {
		api.BadRequest(c, msgs.BadRequest)
		return
	}

	shops, total, err := h.service.ListByOwner(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		api.InternalError(c, msgs.InternalError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"shops": shops, "pagination": api.PaginationMeta(page, pageSize, total)})
}

func (h *Handler) Get(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	id := c.Param("id")
	shop, err := h.service.GetByIDAndOwner(c.Request.Context(), id, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.ShopNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, shop)
}

func (h *Handler) Update(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	id := c.Param("id")
	var input UpdateShopInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	shop, err := h.service.Update(c.Request.Context(), id, userID, input)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.ShopNotFound})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, shop)
}

func (h *Handler) Delete(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id, userID); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.ShopNotFound})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.Deleted})
}
