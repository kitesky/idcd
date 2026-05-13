package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/packages/db/gen/idcdmain"
)

// UserRepository wraps sqlc Queries with domain-friendly error handling.
type UserRepository struct {
	q *idcdmain.Queries
}

// NewUserRepository returns a UserRepository backed by the given pool.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{q: idcdmain.New(pool)}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (idcdmain.User, error) {
	u, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.User{}, ErrNotFound
		}
		return idcdmain.User{}, fmt.Errorf("user.GetByID: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (idcdmain.User, error) {
	u, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.User{}, ErrNotFound
		}
		return idcdmain.User{}, fmt.Errorf("user.GetByEmail: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (idcdmain.User, error) {
	u, err := r.q.GetUserByUsername(ctx, &username)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.User{}, ErrNotFound
		}
		return idcdmain.User{}, fmt.Errorf("user.GetByUsername: %w", err)
	}
	return u, nil
}

func (r *UserRepository) Create(ctx context.Context, p idcdmain.CreateUserParams) (idcdmain.User, error) {
	u, err := r.q.CreateUser(ctx, p)
	if err != nil {
		if isDuplicate(err) {
			return idcdmain.User{}, ErrDuplicate
		}
		return idcdmain.User{}, fmt.Errorf("user.Create: %w", err)
	}
	return u, nil
}

func (r *UserRepository) UpdateStatus(ctx context.Context, id, status string) (idcdmain.User, error) {
	u, err := r.q.UpdateUserStatus(ctx, idcdmain.UpdateUserStatusParams{ID: id, Status: status})
	if err != nil {
		if isNoRows(err) {
			return idcdmain.User{}, ErrNotFound
		}
		return idcdmain.User{}, fmt.Errorf("user.UpdateStatus: %w", err)
	}
	return u, nil
}

func (r *UserRepository) MarkEmailVerified(ctx context.Context, id string) (idcdmain.User, error) {
	u, err := r.q.UpdateUserEmailVerified(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.User{}, ErrNotFound
		}
		return idcdmain.User{}, fmt.Errorf("user.MarkEmailVerified: %w", err)
	}
	return u, nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, p idcdmain.UpdateUserLastLoginParams) error {
	if err := r.q.UpdateUserLastLogin(ctx, p); err != nil {
		return fmt.Errorf("user.UpdateLastLogin: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, p idcdmain.UpdateUserProfileParams) (idcdmain.User, error) {
	u, err := r.q.UpdateUserProfile(ctx, p)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.User{}, ErrNotFound
		}
		return idcdmain.User{}, fmt.Errorf("user.UpdateProfile: %w", err)
	}
	return u, nil
}

func (r *UserRepository) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	err := r.q.UpdateUserPasswordHash(ctx, idcdmain.UpdateUserPasswordHashParams{ID: id, PasswordHash: &hash})
	if err != nil {
		return fmt.Errorf("user.UpdatePasswordHash: %w", err)
	}
	return nil
}

func (r *UserRepository) SoftDelete(ctx context.Context, id string) error {
	if err := r.q.SoftDeleteUser(ctx, id); err != nil {
		return fmt.Errorf("user.SoftDelete: %w", err)
	}
	return nil
}

func (r *UserRepository) CreateCredential(ctx context.Context, p idcdmain.CreateUserCredentialParams) (idcdmain.UserCredential, error) {
	c, err := r.q.CreateUserCredential(ctx, p)
	if err != nil {
		if isDuplicate(err) {
			return idcdmain.UserCredential{}, ErrDuplicate
		}
		return idcdmain.UserCredential{}, fmt.Errorf("user.CreateCredential: %w", err)
	}
	return c, nil
}

func (r *UserRepository) GetCredentialByTypeAndExternal(ctx context.Context, credType, externalID string) (idcdmain.UserCredential, error) {
	c, err := r.q.GetUserCredentialByTypeAndExternal(ctx, idcdmain.GetUserCredentialByTypeAndExternalParams{
		Type:       credType,
		ExternalID: &externalID,
	})
	if err != nil {
		if isNoRows(err) {
			return idcdmain.UserCredential{}, ErrNotFound
		}
		return idcdmain.UserCredential{}, fmt.Errorf("user.GetCredential: %w", err)
	}
	return c, nil
}

func (r *UserRepository) CreateOTP(ctx context.Context, p idcdmain.CreateUserOTPParams) (idcdmain.UserOtp, error) {
	otp, err := r.q.CreateUserOTP(ctx, p)
	if err != nil {
		return idcdmain.UserOtp{}, fmt.Errorf("user.CreateOTP: %w", err)
	}
	return otp, nil
}

func (r *UserRepository) GetOTPByIDAndType(ctx context.Context, id, otpType string) (idcdmain.UserOtp, error) {
	otp, err := r.q.GetUserOTPByIDAndType(ctx, idcdmain.GetUserOTPByIDAndTypeParams{ID: id, Type: otpType})
	if err != nil {
		if isNoRows(err) {
			return idcdmain.UserOtp{}, ErrNotFound
		}
		return idcdmain.UserOtp{}, fmt.Errorf("user.GetOTP: %w", err)
	}
	return otp, nil
}

func (r *UserRepository) MarkOTPUsed(ctx context.Context, id string) error {
	return r.q.MarkUserOTPUsed(ctx, id)
}

func (r *UserRepository) IncrementOTPAttempts(ctx context.Context, id string) error {
	return r.q.IncrementUserOTPAttempts(ctx, id)
}
