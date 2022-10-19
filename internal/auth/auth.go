package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
)

func ValidToken(token string, key string) bool {
	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("error with method")
		}
		return []byte(key), nil
	})
	if err != nil {
		return false
	}
	return t.Valid
}

func GenerateJWT(key string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["authorized"] = true
	claims["client"] = "test-client"
	claims["exp"] = time.Now().Add(time.Minute).Unix()

	tokenString, err := token.SignedString([]byte(key))

	if err != nil {
		return "", err
	}

	return tokenString, nil
}
