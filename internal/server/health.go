package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func apiHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	}
}
