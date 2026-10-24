package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxLoginAttempts = 5
const LoginAttemptWindow = 15 * time.Minute

func loginContactHash(email, phone string) string {
	contact := "phone:" + strings.TrimSpace(phone)
	if email != "" {
		contact = "email:" + strings.ToLower(strings.TrimSpace(email))
	}
	hash := sha256.Sum256([]byte(contact))
	return hex.EncodeToString(hash[:])
}

func ReserveLoginAttempt(pool *pgxpool.Pool, email, phone string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
        INSERT INTO login_attempts (contact_hash, attempts, expires_at)
        VALUES ($1, 1, clock_timestamp() + $3 * interval '1 second')
        ON CONFLICT (contact_hash) DO UPDATE SET
            attempts = CASE WHEN login_attempts.expires_at <= clock_timestamp()
                THEN 1 ELSE login_attempts.attempts + 1 END,
            expires_at = CASE WHEN login_attempts.expires_at <= clock_timestamp()
                THEN EXCLUDED.expires_at ELSE login_attempts.expires_at END
        WHERE login_attempts.expires_at <= clock_timestamp()
            OR login_attempts.attempts < $2
        RETURNING attempts`
	var attempts int
	err := pool.QueryRow(ctx, query, loginContactHash(email, phone), MaxLoginAttempts, LoginAttemptWindow.Seconds()).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func ClearLoginAttempts(pool *pgxpool.Pool, email, phone string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, `DELETE FROM login_attempts WHERE contact_hash = $1`, loginContactHash(email, phone))
	return err
}
