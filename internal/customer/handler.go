package customer

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Create handles POST /customers
func (h *Handler) Create(c *gin.Context) {
	var input CreateCustomerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid json body"})
		return
	}

	customer, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, customer)
}

// Get handles GET /customers/:id
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	customer, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// List handles GET /customers
func (h *Handler) List(c *gin.Context) {
	shopID := c.Query("shop_id")
	hasDebtStr := c.Query("has_debt")

	var hasDebt *bool
	if hasDebtStr != "" {
		val, err := strconv.ParseBool(hasDebtStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "has_debt must be true or false"})
			return
		}
		hasDebt = &val
	}

	customers, err := h.service.List(c.Request.Context(), shopID, hasDebt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"customers": customers})
}

// GetDebtHistory handles GET /customers/:id/debts
func (h *Handler) GetDebtHistory(c *gin.Context) {
	customerID := c.Param("id")
	records, err := h.service.GetDebtHistory(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"debt_records": records})
}

// RecordPayment handles POST /customers/:id/payment
func (h *Handler) RecordPayment(c *gin.Context) {
	customerID := c.Param("id")
	userID, exists := c.Get("auth.userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var input RecordPaymentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid json body"})
		return
	}

	record, err := h.service.RecordPayment(c.Request.Context(), customerID, userID.(string), input)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}
