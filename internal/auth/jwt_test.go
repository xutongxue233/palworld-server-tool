package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

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
