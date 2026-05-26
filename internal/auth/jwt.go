package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims

	Role string `json:"role"`
}

func CreateToken(jwtSecret string, userID string, role string, ttl time.Duration) (string, error) {

	now := time.Now().UTC()
	exp := now.Add(ttl)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},

		Role: role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(jwtSecret))

	if err != nil {
		return "", fmt.Errorf("Sign token failed: %w", err)
	}

	return signed, nil
}

func CreateAccessToken(jwtSecret string, userID string, role string) (string, error) {
	return CreateToken(jwtSecret, userID, role, 60*time.Minute)
}

func CreateRefreshToken(jwtRefreshSecret string, userID string, role string) (string, error) {
	return CreateToken(jwtRefreshSecret, userID, role, 30*24*time.Hour)
}

func ParseToken(jwtSecret string, tokenString string) (Claims, error) {
	var claims Claims

	parsed, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
		}

		return []byte(jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	if err != nil {
		return Claims{}, fmt.Errorf("Parse token failed: %w", err)
	}

	if !parsed.Valid {
		return Claims{}, errors.New("Invalid token")
	}

	if claims.Subject == "" {
		return Claims{}, errors.New("Token missing subject")
	}

	return claims, nil
}
