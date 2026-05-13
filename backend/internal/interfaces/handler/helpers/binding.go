package helpers

import (
	"net/http"

	"image-toolkit/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// BindJSON binds JSON from the request body to target.
// Returns true if binding succeeded, false if error response was written.
func BindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
		return false
	}
	return true
}

// ValidatePassword checks if a password meets length requirements.
func ValidatePassword(password string) bool {
	return len(password) >= PasswordMinLength && len(password) <= PasswordMaxLength
}

// WritePasswordError writes a password validation error response.
func WritePasswordError(c *gin.Context) {
	c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthPasswordLength))
}
