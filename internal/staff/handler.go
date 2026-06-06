package staff

import (
	"net/http"
	"strings"

	"shop_keeper_backend/internal/api"
	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Handler struct {
	service    *Service
	shopLookup ShopLookup
}

func NewHandler(service *Service, shopLookup ShopLookup) *Handler {
	return &Handler{service: service, shopLookup: shopLookup}
}

func (h *Handler) GetMyShop(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	staffMember, err := h.service.GetByID(c.Request.Context(), userID)
	if err != nil || staffMember.ShopID == "" || h.shopLookup == nil {
		c.JSON(http.StatusOK, gin.H{"shop_name": "", "shop_description": ""})
		return
	}

	name, desc, err := h.shopLookup.GetShopSummary(c.Request.Context(), staffMember.ShopID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"shop_name": "", "shop_description": ""})
		return
	}

	c.JSON(http.StatusOK, gin.H{"shop_name": name, "shop_description": desc})
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

	var input CreateStaffInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	if strings.TrimSpace(input.ShopID) == "" && h.shopLookup != nil {
		if shopID, err := h.shopLookup.GetOwnerShopID(c.Request.Context(), userID); err == nil {
			input.ShopID = shopID
		}
	}

	staff, err := h.service.Create(c.Request.Context(), userID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ToPublicStaff(staff))
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

	staffList, total, err := h.service.ListByOwner(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		api.InternalError(c, msgs.InternalError)
		return
	}

	publicStaff := make([]PublicStaff, 0, len(staffList))
	for _, s := range staffList {
		publicStaff = append(publicStaff, ToPublicStaff(s))
	}

	c.JSON(http.StatusOK, gin.H{"staff": publicStaff, "pagination": api.PaginationMeta(page, pageSize, total)})
}

func (h *Handler) Get(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	id := c.Param("id")
	staff, err := h.service.GetByIDAndOwner(c.Request.Context(), id, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.StaffNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, ToPublicStaff(staff))
}

func (h *Handler) Update(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	id := c.Param("id")
	var input UpdateStaffInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	staff, err := h.service.Update(c.Request.Context(), id, userID, input)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.StaffNotFound})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ToPublicStaff(staff))
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
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.StaffNotFound})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.Deleted})
}

func (h *Handler) SaveFCMToken(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	var body struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.MissingRequiredField})
		return
	}

	if err := h.service.SaveFCMToken(c.Request.Context(), userID, body.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.FCMTokenSaved})
}

func (h *Handler) GetCredentials(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	id := c.Param("id")
	credentials, err := h.service.GetCredentials(c.Request.Context(), id, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.StaffNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, credentials)
}
