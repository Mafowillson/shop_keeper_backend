package staff

import (
	"context"
	"fmt"
	"shop_keeper_backend/internal/auth"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// StaffLoginNotifier is satisfied by notification.Service.
// Defined as an interface here to avoid a circular import between the
// staff and notification packages.
type StaffLoginNotifier interface {
	NotifyStaffLogin(ctx context.Context, ownerID, shopID bson.ObjectID, staffName string)
}

// ShopLookup provides shop details without importing the shop package directly.
// Satisfied by shopInfoAdapter in httpserver.
type ShopLookup interface {
	GetShopSummary(ctx context.Context, shopID string) (name, description string, err error)
	// GetOwnerShopID returns the ID of the first active shop owned by ownerID.
	GetOwnerShopID(ctx context.Context, ownerID string) (shopID string, err error)
}

type AuthService struct {
	repo             *Repo
	jwtSecret        string
	jwtRefreshSecret string
	notifier         StaffLoginNotifier
	shopLookup       ShopLookup
}

func NewAuthService(
	repo *Repo,
	jwtSecret, jwtRefreshSecret string,
	notifier StaffLoginNotifier,
	shopLookup ShopLookup,
) *AuthService {
	return &AuthService{
		repo:             repo,
		jwtSecret:        jwtSecret,
		jwtRefreshSecret: jwtRefreshSecret,
		notifier:         notifier,
		shopLookup:       shopLookup,
	}
}

type StaffLoginInput struct {
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
}

type StaffAuthResult struct {
	Token           string      `json:"token"`
	RefreshToken    string      `json:"refresh_token"`
	Staff           PublicStaff `json:"staff"`
	ShopName        string      `json:"shop_name,omitempty"`
	ShopDescription string      `json:"shop_description,omitempty"`
}

type StaffRefreshResult struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

func (as *AuthService) Login(ctx context.Context, input StaffLoginInput) (StaffAuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	phoneNumber := strings.TrimSpace(input.PhoneNumber)

	if email == "" {
		return StaffAuthResult{}, fmt.Errorf("email is required")
	}

	if phoneNumber == "" {
		return StaffAuthResult{}, fmt.Errorf("phone number is required")
	}

	staff, err := as.repo.FindByEmail(ctx, email)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return StaffAuthResult{}, fmt.Errorf("invalid credentials")
		}
		return StaffAuthResult{}, err
	}

	if !staff.IsActive {
		return StaffAuthResult{}, fmt.Errorf("staff account is inactive")
	}

	service := NewService(as.repo)
	if err := service.authenticatePassword(phoneNumber, staff.PasswordHash); err != nil {
		return StaffAuthResult{}, fmt.Errorf("invalid credentials")
	}

	// Auto-heal: if this staff member has no ShopID (created before the
	// mandatory-shop enforcement), derive it from the owner's shop and persist it.
	if strings.TrimSpace(staff.ShopID) == "" && as.shopLookup != nil {
		if shopID, err := as.shopLookup.GetOwnerShopID(ctx, staff.OwnerID); err == nil && shopID != "" {
			if _, err := as.repo.Update(ctx, staff.ID, bson.M{
				"shop_id":    shopID,
				"updated_at": time.Now().UTC(),
			}); err == nil {
				staff.ShopID = shopID
			}
		}
	}

	accessToken, err := auth.CreateAccessToken(as.jwtSecret, staff.ID, "staff")
	if err != nil {
		return StaffAuthResult{}, err
	}

	refreshToken, err := auth.CreateRefreshToken(as.jwtRefreshSecret, staff.ID, "staff")
	if err != nil {
		return StaffAuthResult{}, err
	}

	// Fire staff-login notification to the owner (non-blocking).
	if as.notifier != nil {
		ownerOID, ownerErr := bson.ObjectIDFromHex(staff.OwnerID)
		shopOID, shopErr := bson.ObjectIDFromHex(staff.ShopID)
		if ownerErr == nil && shopErr == nil {
			as.notifier.NotifyStaffLogin(ctx, ownerOID, shopOID, staff.Name)
		}
	}

	result := StaffAuthResult{
		Token:        accessToken,
		RefreshToken: refreshToken,
		Staff:        ToPublicStaff(staff),
	}

	// Attach shop name/description so the mobile client can display them
	// without a separate API call (staff cannot access GET /shops).
	if as.shopLookup != nil && staff.ShopID != "" {
		if name, desc, err := as.shopLookup.GetShopSummary(ctx, staff.ShopID); err == nil {
			result.ShopName = name
			result.ShopDescription = desc
		}
	}

	return result, nil
}

func (as *AuthService) RefreshToken(ctx context.Context, refreshToken string) (StaffRefreshResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return StaffRefreshResult{}, fmt.Errorf("refresh token is required")
	}

	claims, err := auth.ParseToken(as.jwtRefreshSecret, refreshToken)
	if err != nil {
		return StaffRefreshResult{}, fmt.Errorf("invalid refresh token")
	}

	if claims.Role != "staff" {
		return StaffRefreshResult{}, fmt.Errorf("invalid refresh token")
	}

	staffMember, err := as.repo.FindByID(ctx, claims.Subject)
	if err != nil {
		return StaffRefreshResult{}, fmt.Errorf("invalid refresh token")
	}

	if !staffMember.IsActive {
		return StaffRefreshResult{}, fmt.Errorf("staff account is inactive")
	}

	newAccess, err := auth.CreateAccessToken(as.jwtSecret, staffMember.ID, "staff")
	if err != nil {
		return StaffRefreshResult{}, err
	}

	newRefresh, err := auth.CreateRefreshToken(as.jwtRefreshSecret, staffMember.ID, "staff")
	if err != nil {
		return StaffRefreshResult{}, err
	}

	return StaffRefreshResult{Token: newAccess, RefreshToken: newRefresh}, nil
}
