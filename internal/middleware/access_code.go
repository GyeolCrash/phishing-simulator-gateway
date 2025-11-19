package middleware

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func AccessCodeMiddleware() gin.HandlerFunc {
	accessCode := os.Getenv("PROJECT_ACCESS_CODE")
	if accessCode == "" {
		log.Fatal("[Fatal] ACCESS_CODE가 없음")
	}

	return func(c *gin.Context) {
		clientKey := c.GetHeader("Project-Secure")

		if clientKey != accessCode {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid Access Code"})
			return
		}
		c.Next()
	}
}
