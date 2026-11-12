package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// withDB runs fn on a migrated database from DATABASE_URL.
func withDB(fn func(ctx context.Context, q *sqlc.Queries) error) error {
	return withPool(func(ctx context.Context, pool *pgxpool.Pool) error { return fn(ctx, sqlc.New(pool)) })
}

func withPool(fn func(ctx context.Context, pool *pgxpool.Pool) error) error {
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is not set")
	}
	pool, err := db.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	return fn(ctx, pool)
}

// seedCatalog applies the catalog shipped in the image when its version is new, as serve does on start.
func seedCatalog() error {
	return withPool(func(ctx context.Context, pool *pgxpool.Pool) error {
		return catalog.Seed(ctx, pool, catalog.SeedAuto, catalog.SeedDir, cmp.Or(os.Getenv("IMAGES_DIR"), "/data/exercises"))
	})
}

// importExercises reads DATABASE_URL; images go to /data/exercises unless given.
func importExercises(args []string) error {
	images := "/data/exercises"
	if len(args) == 2 {
		images = args[1]
	}
	return withDB(func(ctx context.Context, q *sqlc.Queries) error {
		n, err := catalog.ImportFreeExerciseDB(ctx, q, args[0], images)
		if err != nil {
			return err
		}
		fmt.Printf("imported %d exercises\n", n)
		return nil
	})
}

// importFoods loads a USDA FoodData Central zip, an INDB CSV, the IFCT seed (path optional) or the
// food concepts (path optional). Every food import then reassigns concepts so new rows join theirs.
func importFoods(args []string) error {
	dataset, path := args[0], ""
	if len(args) == 2 {
		path = args[1]
	}
	var imp func(context.Context, *sqlc.Queries, string) (int, error)
	switch dataset {
	case "usda":
		imp = catalog.ImportUSDA
	case "indb":
		imp = catalog.ImportINDB
	case "ifct":
		imp = catalog.ImportIFCT
	case "concepts":
	default:
		return errors.New(usage)
	}
	if path == "" && dataset != "ifct" && dataset != "concepts" {
		return errors.New(usage)
	}
	return withPool(func(ctx context.Context, pool *pgxpool.Pool) error {
		if dataset == "concepts" {
			n, err := catalog.ImportFoodConcepts(ctx, pool, path)
			if err != nil {
				return err
			}
			fmt.Printf("assigned %d foods to concepts\n", n)
			return nil
		}
		n, err := imp(ctx, sqlc.New(pool), path)
		if err != nil {
			return err
		}
		fmt.Printf("imported %d foods\n", n)
		m, err := catalog.ImportFoodConcepts(ctx, pool, "")
		if err != nil {
			return err
		}
		fmt.Printf("assigned %d foods to concepts\n", m)
		return nil
	})
}
