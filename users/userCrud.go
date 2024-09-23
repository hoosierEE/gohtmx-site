package users

import (
	"context"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	Username string    `db:"username"`
	Email    string    `db:"email"`
	Pass     string    `db:"password_hash"`
	Created  time.Time `db:"created_at"`
}

func HashPW(pw string) (string, error) {
	return argon2id.CreateHash(pw, argon2id.DefaultParams)
}

func ComparePW(pw, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(pw, hash)
}

func Exists(pool *pgxpool.Pool, name string) (bool, error) {
	exists := false
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	err := pool.QueryRow(context.Background(), query, name).Scan(&exists)
	return exists, err
}

func Get(pool *pgxpool.Pool, name string) (User, error) {
	query := `SELECT username, email, password_hash, created_at FROM users WHERE username = $1`
	rows, err := pool.Query(context.Background(), query, name)
	if err != nil {
		return User{}, err
	}
	defer rows.Close()
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[User])
}
