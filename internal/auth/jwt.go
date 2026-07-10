package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

const (
	prefixBearer = "Bearer "
	prefixJWT    = "JWT "
)

func signingKey() []byte {
	// Configuration is loaded after package initialization, so the key must be
	// resolved when a token is signed or verified instead of at package load.
	return []byte(viper.GetString("web.password"))
}

func tokenFromHeader(authHeader string) (string, bool) {
	switch {
	case strings.HasPrefix(authHeader, prefixBearer):
		return strings.TrimSpace(strings.TrimPrefix(authHeader, prefixBearer)), true
	case strings.HasPrefix(authHeader, prefixJWT):
		return strings.TrimSpace(strings.TrimPrefix(authHeader, prefixJWT)), true
	default:
		return "", false
	}
}

func parseToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return signingKey(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
}

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := tokenFromHeader(c.GetHeader("Authorization"))
		if !ok || tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - token missing"})
			return
		}

		token, err := parseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - invalid token"})
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("claims", claims)
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - invalid claims"})
			return
		}

		c.Next()
	}
}

func OptionalJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 默认未登录
		c.Set("loggedIn", false)
		tokenString, ok := tokenFromHeader(c.GetHeader("Authorization"))
		if ok && tokenString != "" {
			token, err := parseToken(tokenString)
			if err == nil && token != nil {
				if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
					c.Set("claims", claims)
					c.Set("loggedIn", true)
				}
			}
		}
		c.Next()
	}
}

func GenerateToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString(signingKey())
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
