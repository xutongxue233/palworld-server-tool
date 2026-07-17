package api

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/auth"
	"github.com/zaigie/palworld-server-tool/internal/config"
)

type LoginInfo struct {
	Password string `json:"password"`
}

type AuthStatusResponse struct {
	PasswordConfigured bool `json:"password_configured"`
	PasswordChangeable bool `json:"password_changeable"`
}

type PasswordUpdateRequest struct {
	Password             string `json:"password" binding:"required"`
	PasswordConfirmation string `json:"password_confirmation" binding:"required"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

// authStatusHandler godoc
// @Summary Get Web administrator password status
// @Description Report whether the one-time Web administrator password setup is complete
// @Tags Auth
// @Produce json
// @Success 200 {object} AuthStatusResponse
// @Router /api/auth/status [get]
func authStatusHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, AuthStatusResponse{
		PasswordConfigured: config.WebPasswordConfigured(),
		PasswordChangeable: !config.WebPasswordManagedByEnvironment(),
	})
}

// loginHandler godoc
// @Summary		Login
// @Description	Login
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			login_info	body		LoginInfo	true	"Login Info"
// @Success		200			{object}	TokenResponse
// @Failure		400			{object}	ErrorResponse
// @Failure		401			{object}	ErrorResponse
// @Failure		409			{object}	ErrorResponse
// @Router			/api/login [post]
func loginHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var loginInfo LoginInfo
	if err := c.ShouldBindJSON(&loginInfo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	correctPassword := viper.GetString("web.password")
	if !config.WebPasswordConfigured() {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "web administrator password is not configured",
			Code:  "web_password_not_configured",
		})
		return
	}
	if subtle.ConstantTimeCompare([]byte(loginInfo.Password), []byte(correctPassword)) != 1 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "incorrect password", Code: "incorrect_password"})
		return
	}
	tokenString, err := auth.GenerateToken()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not generate token"})
		return
	}
	c.JSON(http.StatusOK, TokenResponse{Token: tokenString})
}

// initializePasswordHandler godoc
// @Summary Set the initial Web administrator password
// @Description Configure the Web administrator password once when no password exists and return an authenticated token
// @Tags Auth
// @Accept json
// @Produce json
// @Param password body PasswordUpdateRequest true "Initial password"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/auth/password [post]
func initializePasswordHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	request, ok := bindPasswordUpdate(c)
	if !ok {
		return
	}
	if err := config.InitializeWebPassword(request.Password); err != nil {
		writePasswordUpdateError(c, err)
		return
	}
	token, err := auth.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not generate token"})
		return
	}
	c.JSON(http.StatusOK, TokenResponse{Token: token})
}

// changePasswordHandler godoc
// @Summary Change the Web administrator password
// @Description Replace the database-backed Web administrator password immediately and return a token signed with the new password
// @Tags Auth
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param password body PasswordUpdateRequest true "New password"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/auth/password [put]
func changePasswordHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if c.GetBool("fleet_authenticated") {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "fleet node authentication cannot change the Web administrator password",
			Code:  "web_password_user_auth_required",
		})
		return
	}
	request, ok := bindPasswordUpdate(c)
	if !ok {
		return
	}
	if err := config.ChangeWebPassword(request.Password); err != nil {
		writePasswordUpdateError(c, err)
		return
	}
	token, err := auth.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not generate token"})
		return
	}
	c.JSON(http.StatusOK, TokenResponse{Token: token})
}

func bindPasswordUpdate(c *gin.Context) (PasswordUpdateRequest, bool) {
	var request PasswordUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "web_password_invalid"})
		return PasswordUpdateRequest{}, false
	}
	if request.Password != request.PasswordConfirmation {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "password confirmation does not match",
			Code:  "web_password_confirmation_mismatch",
		})
		return PasswordUpdateRequest{}, false
	}
	return request, true
}

func writePasswordUpdateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, config.ErrWebPasswordTooShort):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "web_password_too_short"})
	case errors.Is(err, config.ErrWebPasswordTooLong):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "web_password_too_long"})
	case errors.Is(err, config.ErrWebPasswordAlreadyConfigured):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "web_password_already_configured"})
	case errors.Is(err, config.ErrWebPasswordNotConfigured):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "web_password_not_configured"})
	case errors.Is(err, config.ErrWebPasswordManagedByEnv):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "web_password_managed_by_environment"})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
}
