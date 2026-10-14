package handlers

import (
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetCases(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		cases, err := repository.GetAllCases(pool)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		c.JSON(http.StatusOK, gin.H{"cases": cases})
	}

}

func CreateCase(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		var req models.CreateCase

		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Parameters missing or bad"})
			return
		}

		caseID, err := repository.CreateCase(pool, req)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		c.JSON(http.StatusOK, caseID)

	}
}

func UpdateReportsCase(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, ok := requireActiveStaff(c, pool)

		if !ok {
			return
		}

		folio := c.Param("folio")

		var req models.UpdateCase
		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No case ID provided"})
			return
		}

		response, err := repository.UpdateReportCase(pool, req, folio)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"errsor": err})
			return
		}

		c.JSON(http.StatusOK, response)
	}
}
