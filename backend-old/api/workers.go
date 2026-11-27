package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/service/data"
)

func (c *Controller) listWorkers(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveWorkers(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showWorker(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.DataSvc.RetrieveWorkerByID(ctx, id)
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
