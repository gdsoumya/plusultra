package auth

import (
	"cmp"
	"context"
	"errors"
	"regexp"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

var usernameFormat = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

const (
	minPasswordLen = 10   // characters
	maxPasswordLen = 1024 // bytes
)

// NewUser is the input of Register. Empty DisplayName, Units and Timezone
// default to the username, "kg" and "UTC".
type NewUser struct {
	Username, Password, InviteCode string
	DisplayName, Units, Timezone   string
}

// Register creates a user. The first user becomes admin. In invite mode every
// later user needs an unused, unexpired invite code, consumed on success.
func (s *Service) Register(ctx context.Context, nu NewUser) (sqlc.User, error) {
	if !usernameFormat.MatchString(nu.Username) {
		return sqlc.User{}, invalidArgument("username must be 3 to 32 characters of a-z, 0-9 or _")
	}
	if err := checkPassword(nu.Password); err != nil {
		return sqlc.User{}, err
	}
	tz := cmp.Or(nu.Timezone, "UTC")
	if err := CheckTimezone(tz); err != nil {
		return sqlc.User{}, err
	}
	hash, err := HashPassword(ctx, nu.Password)
	if err != nil {
		return sqlc.User{}, s.publicError(err)
	}

	var user sqlc.User
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.Q.WithTx(tx)
		if err := q.LockRegistration(ctx); err != nil {
			return err
		}
		n, err := q.CountUsers(ctx)
		if err != nil {
			return err
		}
		needInvite := n > 0 && s.cfg.Registration != RegistrationOpen
		// Checked before the username so callers without an invite cannot probe for taken names.
		if needInvite {
			_, err := q.GetUsableInvite(ctx, nu.InviteCode)
			if errors.Is(err, pgx.ErrNoRows) {
				return permissionDenied("a valid invite code is required")
			}
			if err != nil {
				return err
			}
		}
		user, err = q.CreateUser(ctx, sqlc.CreateUserParams{
			Username: nu.Username, PasswordHash: hash, DisplayName: cmp.Or(nu.DisplayName, nu.Username),
			Units: cmp.Or(nu.Units, "kg"), Timezone: tz, IsAdmin: n == 0,
		})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return connect.NewError(connect.CodeAlreadyExists, errors.New("username is taken"))
		}
		if err != nil {
			return err
		}
		if needInvite {
			_, err = q.ConsumeInvite(ctx, sqlc.ConsumeInviteParams{UsedBy: &user.ID, Code: nu.InviteCode})
		}
		return err
	})
	if err != nil {
		return sqlc.User{}, s.publicError(err)
	}
	return user, nil
}

// CheckTimezone accepts IANA zone names such as "Asia/Kolkata" and "UTC".
func CheckTimezone(tz string) error {
	// LoadLocation also accepts "" and "Local", which are not IANA names.
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" {
		return invalidArgument("unknown time zone")
	}
	return nil
}

func checkPassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordLen || len(password) > maxPasswordLen {
		return invalidArgument("password must be at least 10 characters and at most 1024 bytes")
	}
	return nil
}
