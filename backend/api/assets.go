package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
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

func (c *Controller) updateCamera(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		newError(ctx, http.StatusBadRequest, fmt.Errorf("camera name is missing"))
		return
	}

	var camera model.Camera
	err := ctx.BindJSON(&camera)
	if err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	err = c.Svcs.DataSvc.UpdateCamera(ctx, camera)
	if err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	ctx.JSON(http.StatusOK, nil)
}
