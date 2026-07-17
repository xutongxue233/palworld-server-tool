package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/fleet"
)

const (
	prefixBearer = "Bearer "
	prefixJWT    = "JWT "
)

var ErrPasswordNotConfigured = errors.New("web administrator password is not configured")

func signingKey() ([]byte, error) {
	// Configuration is loaded after package initialization, so the key must be
	// resolved when a token is signed or verified instead of at package load.
	password := viper.GetString("web.password")
	if strings.TrimSpace(password) == "" {
		return nil, ErrPasswordNotConfigured
	}
	return []byte(password), nil
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
		return signingKey()
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
}

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if fleetTokenMatches(c.GetHeader(fleet.NodeTokenHeader)) {
			c.Set("fleet_authenticated", true)
			c.Set("loggedIn", true)
			c.Next()
			return
		}
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
		if fleetTokenMatches(c.GetHeader(fleet.NodeTokenHeader)) {
			c.Set("fleet_authenticated", true)
			c.Set("loggedIn", true)
			c.Next()
			return
		}
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

func FleetNodeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := viper.GetString("fleet.node_token")
		if !fleet.ValidTokenSecret(expected) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "fleet node authentication is not configured",
				"code":  "fleet_node_auth_not_configured",
			})
			return
		}
		if !fleetTokenMatches(c.GetHeader(fleet.NodeTokenHeader)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized - invalid fleet node token",
				"code":  "fleet_node_auth_invalid",
			})
			return
		}
		c.Set("fleet_authenticated", true)
		c.Set("loggedIn", true)
		c.Next()
	}
}

func fleetTokenMatches(provided string) bool {
	expected := viper.GetString("fleet.node_token")
	if !fleet.ValidTokenSecret(expected) || !fleet.ValidTokenSecret(provided) || len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func GenerateToken() (string, error) {
	key, err := signingKey()
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
