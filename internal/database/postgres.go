package database

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect crea un pool de conexiones a PostgreSQL a partir de databaseURL y verifica
// la conexión con un ping. Si el ping falla, cierra el pool y regresa el error.
// El llamador es responsable de cerrar el pool con [pgxpool.Pool.Close].
func Connect(databaseURL string) (*pgxpool.Pool, error) {
	var ctx context.Context = context.Background()

	var config *pgxpool.Config
	var err error

	config, err = pgxpool.ParseConfig(databaseURL)

	if err != nil {
		log.Printf("Unable to parse DATABASE_URL: %v", err)
		return nil, err
	}
	var pool *pgxpool.Pool
	pool, err = pgxpool.NewWithConfig(ctx, config)

	if err != nil {
		log.Printf("Unable to create connection pool: %v", err)
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		log.Printf("Unable to create connection pool: %v", err)
		pool.Close()
		return nil, err
	}

	log.Printf("Succesfully connected to postgres")
	return pool, nil

}
