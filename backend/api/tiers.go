package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/service/data"
)

func (c *Controller) listTiers(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveTiers(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showTier(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.DataSvc.RetrieveTierByID(ctx, id)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}
