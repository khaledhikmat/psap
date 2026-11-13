package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (c *Controller) listErrors(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []interface{}{})
}
