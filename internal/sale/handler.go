package sale

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

func (h *Handler) Create(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input CreateSaleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	sale, err := h.service.Create(c.Request.Context(), userID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sale)
}

func (h *Handler) Get(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	id := c.Param("id")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	sale, err := h.service.GetByIDAndOwner(c.Request.Context(), id, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.SaleNotFound})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sale)
}

func (h *Handler) List(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	shopID := c.Query("shop_id")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		api.Unauthorized(c, msgs.Unauthorized)
		return
	}

	page, pageSize, err := api.ParsePagination(c)
	if err != nil {
		api.BadRequest(c, msgs.BadRequest)
		return
	}

	sales, total, err := h.service.ListByOwner(c.Request.Context(), userID, shopID, page, pageSize)
	if err != nil {
		api.InternalError(c, msgs.InternalError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"sales": sales, "pagination": api.PaginationMeta(page, pageSize, total)})
}
