package handlers

import (
	"go-service-sipinna/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetZones returns every zone to 'administrador'; 'alimentador' only gets its assigned zone.
func GetZones(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		zones, err := repository.GetZones(pool, scopeZoneID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load zones"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"zones": zones})
	}
}
