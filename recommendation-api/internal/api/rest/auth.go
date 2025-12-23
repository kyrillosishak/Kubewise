// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// User represents an authenticated user
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Permission represents a user permission
type Permission string

// LoginRequest is the request body for login
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is the response for successful login
type LoginResponse struct {
	User        User         `json:"user"`
	Token       string       `json:"token"`
	ExpiresIn   int          `json:"expiresIn"` // seconds
	Permissions []Permission `json:"permissions"`
}

// MeResponse is the response for the /auth/me endpoint
type MeResponse struct {
	User        User         `json:"user"`
	Permissions []Permission `json:"permissions"`
}

// loginHandler handles user authentication
func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Code:  "BAD_REQUEST",
		})
		return
	}

	// Authenticate user via auth service
	user, token, err := getAuthStore().Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Invalid credentials",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	permissions, _ := getAuthStore().GetPermissions(c.Request.Context(), user.ID)

	c.JSON(http.StatusOK, LoginResponse{
		User:        *user,
		Token:       token,
		ExpiresIn:   3600, // 1 hour
		Permissions: permissions,
	})
}

// meHandler returns the current authenticated user
func meHandler(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Not authenticated",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	user, err := getAuthStore().GetUser(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not found",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	permissions, _ := getAuthStore().GetPermissions(c.Request.Context(), user.ID)

	c.JSON(http.StatusOK, MeResponse{
		User:        *user,
		Permissions: permissions,
	})
}

// AuthMiddleware validates JWT tokens and sets user context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Error: "Missing authorization header",
				Code:  "UNAUTHORIZED",
			})
			return
		}

		// Remove "Bearer " prefix if present
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// Validate token
		userID, err := getAuthStore().ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Error: "Invalid or expired token",
				Code:  "UNAUTHORIZED",
			})
			return
		}

		c.Set("userID", userID)
		c.Set("token", token)
		c.Next()
	}
}

// generateToken creates a JWT token for a user (placeholder - implement with real JWT)
func generateToken(userID string, expiry time.Duration) (string, error) {
	// TODO: Implement proper JWT token generation
	// For now, return a placeholder
	return "jwt-token-" + userID, nil
}
