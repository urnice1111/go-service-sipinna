package middleware

import (
	"go-service-sipinna/internal/repository"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// auditedPrefixes define las rutas incluidas en la bitácora
var auditedPrefixes = []string{"/report", "/cases", "/admin"}

// AuditLog registra peticiones del personal después de responder
// Guarda usuario, ruta, método, IP y estatus HTTP
func AuditLog(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if c.GetString("user_type") == "citizen" {
			return
		}
		userID, err := uuid.Parse(c.GetString("user_id"))
		if err != nil {
			return
		}
		path := c.Request.URL.Path
		audited := false
		for _, prefix := range auditedPrefixes {
			if strings.HasPrefix(path, prefix) {
				audited = true
				break
			}
		}
		if !audited {
			return
		}

		method, status, ip := c.Request.Method, c.Writer.Status(), c.ClientIP()
		go func() {
			if err := repository.InsertLogEntry(pool, userID, method, path, status, ip); err != nil {
				log.Printf("could not write audit log: %v", err)
			}
		}()
	}
}
