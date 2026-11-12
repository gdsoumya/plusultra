package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrate applies pending migrations and exits; serve does the same on start.
func migrate() error {
	return withPool(func(context.Context, *pgxpool.Pool) error {
		fmt.Println("migrations applied")
		return nil
	})
}
