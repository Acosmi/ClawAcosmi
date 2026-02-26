// Package apierr provides unified API error responses.
// All error responses use {"detail": "..."} format.
// Internal errors are logged but never exposed to clients (BUG-07 fix).
package apierr

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Respond sends a JSON error response with the given status and user-safe message.
func Respond(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"detail": msg})
}

// BadRequest sends a 400 response with a user-safe message.
func BadRequest(c *gin.Context, msg string) {
	Respond(c, http.StatusBadRequest, msg)
}

// NotFound sends a 404 response.
func NotFound(c *gin.Context, msg string) {
	Respond(c, http.StatusNotFound, msg)
}

// Forbidden sends a 403 response.
func Forbidden(c *gin.Context, msg string) {
	Respond(c, http.StatusForbidden, msg)
}

// Unauthorized sends a 401 response.
func Unauthorized(c *gin.Context, msg string) {
	Respond(c, http.StatusUnauthorized, msg)
}

// Internal sends a 500 response with a generic message.
// The actual error is logged server-side but never exposed to the client.
func Internal(c *gin.Context, err error, userMsg string) {
	if err != nil {
		log.Printf("[ERROR] %s: %v", userMsg, err)
	}
	Respond(c, http.StatusInternalServerError, userMsg)
}
