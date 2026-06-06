package customer

import (
	"net/http"
	"strconv"

	"shop_keeper_backend/internal/api"
	"shop_keeper_backend/internal/i18n"

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

	var input CreateCustomerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	customer, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, customer)
}

func (h *Handler) Get(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	id := c.Param("id")
	customer, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.CustomerNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, customer)
}

func (h *Handler) List(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	shopID := c.Query("shop_id")
	hasDebtStr := c.Query("has_debt")

	var hasDebt *bool
	if hasDebtStr != "" {
		val, err := strconv.ParseBool(hasDebtStr)
		if err != nil {
			api.BadRequest(c, msgs.BadRequest)
			return
		}
		hasDebt = &val
	}

	page, pageSize, err := api.ParsePagination(c)
	if err != nil {
		api.BadRequest(c, msgs.BadRequest)
		return
	}

	customers, total, err := h.service.List(c.Request.Context(), shopID, hasDebt, page, pageSize)
	if err != nil {
		api.InternalError(c, msgs.InternalError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"customers": customers, "pagination": api.PaginationMeta(page, pageSize, total)})
}

func (h *Handler) GetDebtHistory(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	customerID := c.Param("id")
	records, err := h.service.GetDebtHistory(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_ = msgs // no localized string needed for this success path
	c.JSON(http.StatusOK, gin.H{"debt_records": records})
}

func (h *Handler) RecordPayment(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	customerID := c.Param("id")
	userID, exists := c.Get("auth.userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	var input RecordPaymentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	record, err := h.service.RecordPayment(c.Request.Context(), customerID, userID.(string), input)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.CustomerNotFound})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}
