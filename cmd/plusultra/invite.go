package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// invite prints a single-use registration code issued by the oldest admin, so
// an operator can add users or recover access without a session.
func invite(args []string) error {
	fs := flag.NewFlagSet("invite", flag.ContinueOnError)
	ttl := fs.Duration("expires-in", 7*24*time.Hour, "how long the code stays valid")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return withPool(func(ctx context.Context, pool *pgxpool.Pool) error {
		users, err := sqlc.New(pool).ListUsers(ctx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(users, func(u sqlc.User) bool { return u.IsAdmin })
		if i < 0 {
			return errors.New("no admin yet: the first user registers without an invite")
		}
		admin := auth.Principal{UserID: users[i].ID.String(), Admin: true}
		inv, err := auth.NewService(pool, auth.Config{}).CreateInvite(ctx, admin, time.Now().Add(*ttl))
		if err != nil {
			return err
		}
		fmt.Println(inv.Code)
		return nil
	})
}
