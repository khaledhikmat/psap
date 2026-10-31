package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
)

// Question Categories

func (c *Controller) listQuestionCategories(ctx *gin.Context) {
	// Support optional type filter
	categoryType := ctx.Query("type")

	var result []model.QuestionCategory
	var err error

	if categoryType != "" {
		result, err = c.Svcs.DataSvc.RetrieveQuestionCategoriesByType(ctx, model.QuestionCategoryType(categoryType))
	} else {
		result, err = c.Svcs.DataSvc.RetrieveQuestionCategories(ctx)
	}

	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showQuestionCategory(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveQuestionCategoryByID(ctx, id)
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

func (c *Controller) listCategoryQuestions(ctx *gin.Context) {
	categoryID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveQuestionsByCategoryID(ctx, categoryID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// Questions

func (c *Controller) listQuestions(ctx *gin.Context) {
	// Support optional categoryType filter
	categoryType := ctx.Query("categoryType")

	var result []model.Question
	var err error

	if categoryType != "" {
		result, err = c.Svcs.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.QuestionCategoryType(categoryType))
	} else {
		result, err = c.Svcs.DataSvc.RetrieveQuestions(ctx)
	}

	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showQuestion(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveQuestionByID(ctx, id)
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
