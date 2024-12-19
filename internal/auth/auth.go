package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPasswordHash(password string, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Issuer:    "chirpy",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(expiresIn).Unix(),
		Subject:   userID.String(),
	}).SignedString([]byte(os.Getenv("JWT_SIGNING_KEY")))
	if err != nil {
		log.Printf("Error while creating and signing JWT")
		return "", err
	}
	return token, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(tokenSecret), nil
	})

	if err != nil {
		return uuid.Nil, fmt.Errorf("parsing token: %w", err)
	}

	if !token.Valid {
		return uuid.Nil, fmt.Errorf("token validation failed")
	}

	sub, ok := claims["sub"]
	if !ok {
		return uuid.Nil, fmt.Errorf("missing subject claim")
	}

	subStr, ok := sub.(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("subject claim is not a string")
	}

	userID, err := uuid.Parse(subStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parsing user ID: %w", err)
	}

	return userID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	auth := headers.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("authorization header missing")
	}
	if !strings.Contains(auth, "Bearer ") {
		return "", fmt.Errorf("invalid Bearer token format")
	}

	token := strings.Split(auth, " ")[1]
	return token, nil
}

func MakeRefreshToken() (string, error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
