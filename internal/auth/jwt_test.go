package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

func TestGenerateTokenRejectsMissingPassword(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	if _, err := GenerateToken(); !errors.Is(err, ErrPasswordNotConfigured) {
		t.Fatalf("missing-password error = %v", err)
	}
}

func TestJWTUsesRuntimeConfigurationAndHS256Only(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "first-secret")
	t.Cleanup(viper.Reset)

	token, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.GET("/required", JWTAuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/optional", OptionalJWTMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"loggedIn": c.GetBool("loggedIn")})
	})

	request := httptest.NewRequest(http.MethodGet, "/required", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid token rejected with status %d", response.Code)
	}

	viper.Set("web.password", "second-secret")
	request = httptest.NewRequest(http.MethodGet, "/required", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("token signed with the old runtime key was accepted: %d", response.Code)
	}

	wrongAlgorithm := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	wrongAlgorithmToken, err := wrongAlgorithm.SignedString([]byte("second-secret"))
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/optional", nil)
	request.Header.Set("Authorization", "Bearer "+wrongAlgorithmToken)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Body.String() != `{"loggedIn":false}` {
		t.Fatalf("unexpected optional auth response: %s", response.Body.String())
	}
}

func TestFleetNodeTokenAuthenticatesRequiredAndOptionalRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "web-secret")
	viper.Set("fleet.node_token", "0123456789abcdef0123456789abcdef")
	t.Cleanup(viper.Reset)

	router := gin.New()
	router.GET("/required", JWTAuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"fleet": c.GetBool("fleet_authenticated")})
	})
	router.GET("/optional", OptionalJWTMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"fleet":    c.GetBool("fleet_authenticated"),
			"loggedIn": c.GetBool("loggedIn"),
		})
	})
	router.GET("/node", FleetNodeAuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for _, path := range []string{"/required", "/optional", "/node"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-PST-Fleet-Token", "0123456789abcdef0123456789abcdef")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code < http.StatusOK || response.Code >= http.StatusMultipleChoices {
			t.Fatalf("valid fleet token rejected by %s with %d: %s", path, response.Code, response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/required", nil)
	request.Header.Set("X-PST-Fleet-Token", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid fleet token was accepted: %d", response.Code)
	}

	viper.Set("fleet.node_token", "short")
	request = httptest.NewRequest(http.MethodGet, "/node", nil)
	request.Header.Set("X-PST-Fleet-Token", "0123456789abcdef0123456789abcdef")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid local fleet token configuration returned %d", response.Code)
	}
}
