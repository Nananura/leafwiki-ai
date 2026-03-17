package auth

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
)

func ApiKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" && parts[1] == apiKey {
			// API key matched, create a virtual admin user
			adminUser := &auth.User{
				ID:       "system_api", // Unique ID
				Username: "API Config",
				Email:    "api@system.local",
				Role:     auth.RoleAdmin,
			}
			c.Set("user", adminUser)
			c.Set("api_key_auth", true)
			log.Printf("Successful authentication via API Key")
		}

		c.Next()
	}
}
