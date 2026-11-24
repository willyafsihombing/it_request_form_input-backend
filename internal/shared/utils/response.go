package utils

import (
	"github.com/gin-gonic/gin"
)

func OK(c *gin.Context, data interface{}) {
	c.JSON(200, data)
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(201, data)
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(400, gin.H{"error": message})
}

func Forbidden(c *gin.Context, message string) {
	c.JSON(403, gin.H{"error": message})
}

func TooManyRequests(c *gin.Context, message string) {
	c.JSON(429, gin.H{"error": message})
}

func NotFound(c *gin.Context, message string) {
	c.JSON(404, gin.H{"error": message})
}

func Conflict(c *gin.Context, message string) {
	c.JSON(409, gin.H{"error": message})
}

func InternalError(c *gin.Context, message string) {
	c.JSON(500, gin.H{"error": message})
}
