package handlers

import (
	"go-service-sipinna/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetZones maneja GET /zones: regresa todas las zonas al 'administrador'; el
// 'alimentador' solo recibe su zona asignada.
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
