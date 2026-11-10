package database

import (
	"errors"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	mssql "github.com/microsoft/go-mssqldb"
	"gorm.io/gorm"
)

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func IsUnique(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	var msErr mssql.Error
	if errors.As(err, &msErr) && (msErr.Number == 2627 || msErr.Number == 2601) {
		return true
	}
	text := err.Error()
	return strings.Contains(text, "UNIQUE constraint failed") || strings.Contains(text, "unique constraint")
}
