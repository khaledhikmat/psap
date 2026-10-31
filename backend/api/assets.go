package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/service/data"
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

func (c *Controller) showAsset(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveAssetByID(ctx, id)
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

func (c *Controller) listAssetLaunches(ctx *gin.Context) {
	assetID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveLaunchesByAssetID(ctx, assetID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listAssetShields(ctx *gin.Context) {
	assetID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveShieldsByAssetID(ctx, assetID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listAssetBaselines(ctx *gin.Context) {
	assetID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveBaselinesByAssetID(ctx, assetID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}
