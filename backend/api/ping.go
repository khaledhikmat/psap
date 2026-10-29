package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	version = "1.0.0"
)

func (c *Controller) ping(ctx *gin.Context) {
	result := struct {
		Message string `json:"message" example:"pong"`
		Version string `json:"version" example:"1.0.0"`
	}{
		Message: "pong",
		Version: version,
	}

	ctx.JSON(http.StatusOK, result)
}
