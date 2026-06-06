package product

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

	var input CreateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	product, err := h.service.Create(c.Request.Context(), input, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func (h *Handler) List(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	shopID := c.Query("shop_id")
	category := c.Query("category")
	search := c.Query("search")

	page, pageSize, err := api.ParsePagination(c)
	if err != nil {
		api.BadRequest(c, msgs.BadRequest)
		return
	}

	products, total, err := h.service.List(c.Request.Context(), shopID, category, search, page, pageSize)
	if err != nil {
		api.InternalError(c, msgs.InternalError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"products": products, "pagination": api.PaginationMeta(page, pageSize, total)})
}

func (h *Handler) Get(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	id := c.Param("id")
	product, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.ProductNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, product)
}

func (h *Handler) Update(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	id := c.Param("id")
	var input UpdateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	product, err := h.service.Update(c.Request.Context(), id, input, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.ProductNotFound})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, product)
}

func (h *Handler) Delete(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	id := c.Param("id")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id, userID); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.ProductNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.ProductDeleted})
}

func (h *Handler) Sync(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input SyncProductsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	products, err := h.service.Sync(c.Request.Context(), input, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"products": products, "message": msgs.SyncCompleted})
}
