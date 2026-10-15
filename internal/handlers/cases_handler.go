package handlers

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetCases(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		cases, err := repository.GetAllCases(pool, scopeZoneID)

		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report or case not found in the allowed zone"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"cases": cases})
	}

}

func CreateCase(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		var req models.CreateCase

		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Parameters missing or bad"})
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}
		created, err := repository.CreateCase(pool, req, scopeZoneID)

		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report or case not found in the allowed zone"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"case": created})

	}
}

func UpdateReportsCase(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)

		if !ok {
			return
		}

		folio := c.Param("report_ref")

		var req models.UpdateCase
		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No case ID provided"})
			return
		}

		response, err := repository.UpdateReportCase(pool, req, folio, scopeZoneID)

		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report or case not found in the allowed zone"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, response)
	}
}
