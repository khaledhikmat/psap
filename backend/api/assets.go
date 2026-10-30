package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// This returns all assets and let the UI filter
func (c *Controller) listAssets(ctx *gin.Context) {
	result, err := c.Svcs.DataSvc.RetrieveAssets(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}
