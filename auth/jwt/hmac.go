package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type hmacIssuer struct {
	secretKey []byte
}

func NewHMACIssuer(secretKey string) (*hmacIssuer, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("secret key cannot be empty")
	}
	return &hmacIssuer{secretKey: []byte(secretKey)}, nil
}

func (i *hmacIssuer) Issue(ctx context.Context, accountID uuid.UUID, username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      accountID.String(),
		"iss":      "HMAC",
		"username": username,
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	return token.SignedString(i.secretKey)
}

func (i *hmacIssuer) Validate(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return i.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		sub, _ := claims["sub"].(string)
		username, _ := claims["username"].(string)

		accountID, err := uuid.Parse(sub)
		if err != nil {
			return nil, fmt.Errorf("invalid account ID in token")
		}

		var expiryTime time.Time
		if exp, ok := claims["exp"].(float64); ok {
			expiryTime = time.Unix(int64(exp), 0)
		}

		return &Claims{
			AccountID:  accountID,
			Username:   username,
			ExpiryTime: expiryTime,
		}, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}
