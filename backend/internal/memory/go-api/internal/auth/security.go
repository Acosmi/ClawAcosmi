// Package auth provides JWT token management and password hashing.
// Mirrors Python core/security.py.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/uhms/go-api/internal/config"
)

// JWT issuer and audience constants.
const (
	jwtIssuer   = "uhms-go-api"
	jwtAudience = "uhms-client"
)

// --- Errors ---

var (
	ErrInvalidToken  = errors.New("invalid or expired token")
	ErrMissingClaims = errors.New("token missing required claims")
)

// --- Password Hashing (bcrypt) ---

// HashPassword hashes a plain-text password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a plain password against a bcrypt hash.
// Returns true if the password matches.
func VerifyPassword(plainPassword, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	return err == nil
}

// --- JWT Token Management ---

// TokenData represents the decoded JWT payload.
type TokenData struct {
	UserID string `json:"sub"`
	Role   string `json:"role"`
}

// Claims extends jwt.RegisteredClaims with custom fields.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// CreateAccessToken creates a JWT access token with the given subject and role.
func CreateAccessToken(subject string, role string, expiresDelta ...time.Duration) (string, error) {
	cfg := config.Get()

	var expiry time.Duration
	if len(expiresDelta) > 0 && expiresDelta[0] > 0 {
		expiry = expiresDelta[0]
	} else {
		expiry = time.Duration(cfg.AccessTokenExpireMin) * time.Minute
	}

	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(cfg.JWTSecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// VerifyToken verifies and decodes a JWT token string.
// Returns the token data or an error if validation fails.
func VerifyToken(tokenString string) (*TokenData, error) {
	cfg := config.Get()

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.JWTSecretKey), nil
	},
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
	)
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return nil, ErrMissingClaims
	}

	role := claims.Role
	if role == "" {
		role = "user"
	}

	return &TokenData{
		UserID: subject,
		Role:   role,
	}, nil
}
