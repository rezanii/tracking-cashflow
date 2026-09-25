package utils

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secret     []byte
	expiration time.Duration
}

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func NewTokenManager(secret string, expiration time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), expiration: expiration}
}

// Generate signs an HS256 token whose subject is the user id.
func (m *TokenManager) Generate(userID int64, email string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.expiration)

	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse validates the signature, the expiry and the signing method, then returns the claims.
func (m *TokenManager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", token.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, Unauthorized("Invalid or expired token")
	}
	return claims, nil
}

func (c *Claims) UserID() (int64, error) {
	userID, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil {
		return 0, Unauthorized("Invalid token subject")
	}
	return userID, nil
}
