package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shop_keeper_backend/internal/auth"
	"shop_keeper_backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

const testSecret = "middleware-unit-test-secret-key!"

func init() {
	gin.SetMode(gin.TestMode)
}

// buildRouter sets up a Gin router with AuthRequired and an optional extra
// middleware (e.g. RequireOwner), then registers GET /test returning 200.
func buildRouter(secret string, extra ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(middleware.AuthRequired(secret))
	for _, mw := range extra {
		r.Use(mw)
	}
	r.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body["error"]
}

// ── AuthRequired ──────────────────────────────────────────────────────────────

func TestAuthRequired_MissingAuthHeader_Returns401(t *testing.T) {
	r := buildRouter(testSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAuthRequired_EmptyBearerToken_Returns401(t *testing.T) {
	r := buildRouter(testSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAuthRequired_InvalidScheme_Returns401(t *testing.T) {
	r := buildRouter(testSecret)
	token, _ := auth.CreateAccessToken(testSecret, "user1", "owner")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAuthRequired_ValidOwnerToken_Returns200(t *testing.T) {
	r := buildRouter(testSecret)
	token, _ := auth.CreateAccessToken(testSecret, "ownerID", "owner")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestAuthRequired_ValidStaffToken_Returns200(t *testing.T) {
	r := buildRouter(testSecret)
	token, _ := auth.CreateAccessToken(testSecret, "staff-uuid-xyz", "staff")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestAuthRequired_ExpiredToken_Returns401(t *testing.T) {
	r := buildRouter(testSecret)
	expired, _ := auth.CreateToken(testSecret, "user1", "owner", -1*time.Second)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+expired)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAuthRequired_TamperedToken_Returns401(t *testing.T) {
	r := buildRouter(testSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer header.payload.badsignature")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAuthRequired_WrongSecret_Returns401(t *testing.T) {
	r := buildRouter(testSecret)
	// Token signed with a *different* secret.
	token, _ := auth.CreateAccessToken("a-completely-different-secret!!", "user1", "owner")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// ── RequireOwner — RBAC ───────────────────────────────────────────────────────

func TestRequireOwner_OwnerRole_Returns200(t *testing.T) {
	r := buildRouter(testSecret, middleware.RequireOwner())
	token, _ := auth.CreateAccessToken(testSecret, "ownerID", "owner")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200 for owner role, got %d", w.Code)
	}
}

func TestRequireOwner_StaffRole_Returns403(t *testing.T) {
	r := buildRouter(testSecret, middleware.RequireOwner())
	token, _ := auth.CreateAccessToken(testSecret, "staffID", "staff")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403 for staff accessing owner route, got %d", w.Code)
	}
	if msg := errorBody(t, w); msg == "" {
		t.Error("response body must contain an 'error' field")
	}
}

func TestRequireOwner_StaffRole_ErrorMessagePresent(t *testing.T) {
	r := buildRouter(testSecret, middleware.RequireOwner())
	token, _ := auth.CreateAccessToken(testSecret, "staffID", "staff")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if errorBody(t, w) == "" {
		t.Error("403 response must include an error message")
	}
}

// ── RequireStaff — RBAC ───────────────────────────────────────────────────────

func TestRequireStaff_StaffRole_Returns200(t *testing.T) {
	r := buildRouter(testSecret, middleware.RequireStaff())
	token, _ := auth.CreateAccessToken(testSecret, "staffID", "staff")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200 for staff role on staff route, got %d", w.Code)
	}
}

func TestRequireStaff_OwnerRole_Returns403(t *testing.T) {
	r := buildRouter(testSecret, middleware.RequireStaff())
	token, _ := auth.CreateAccessToken(testSecret, "ownerID", "owner")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403 for owner accessing staff-only route, got %d", w.Code)
	}
}

// ── GetUserID / GetRole helpers ───────────────────────────────────────────────

func TestGetUserID_AfterValidAuth_ReturnsSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthRequired(testSecret))

	var extractedID string
	r.GET("/check", func(c *gin.Context) {
		id, ok := middleware.GetUserID(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		extractedID = id
		c.Status(http.StatusOK)
	})

	token, _ := auth.CreateAccessToken(testSecret, "expected-user-id", "owner")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handler returned %d", w.Code)
	}
	if extractedID != "expected-user-id" {
		t.Errorf("want %q, got %q", "expected-user-id", extractedID)
	}
}

func TestGetRole_AfterValidAuth_ReturnsRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthRequired(testSecret))

	var extractedRole string
	r.GET("/check", func(c *gin.Context) {
		role, ok := middleware.GetRole(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		extractedRole = role
		c.Status(http.StatusOK)
	})

	token, _ := auth.CreateAccessToken(testSecret, "user1", "owner")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if extractedRole != "owner" {
		t.Errorf("want role %q, got %q", "owner", extractedRole)
	}
}
