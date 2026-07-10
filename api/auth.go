package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/auth"
)

type LoginInfo struct {
	Password string `json:"password"`
}

// loginHandler godoc
// @Summary		Login
// @Description	Login
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			login_info	body		LoginInfo	true	"Login Info"
// @Success		200			{object}	SuccessResponse
// @Failure		400			{object}	ErrorResponse
// @Failure		401			{object}	ErrorResponse
// @Router			/api/login [post]
func loginHandler(c *gin.Context) {
	var loginInfo LoginInfo
	if err := c.ShouldBindJSON(&loginInfo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	correctPassword := viper.GetString("web.password")
	if subtle.ConstantTimeCompare([]byte(loginInfo.Password), []byte(correctPassword)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect password"})
		return
	}
	tokenString, err := auth.GenerateToken()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}
