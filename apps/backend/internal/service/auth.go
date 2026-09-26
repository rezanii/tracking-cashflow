package service

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// bcryptCost is above the library default to slow down offline cracking while staying
// well inside a request budget.
const bcryptCost = 12

type AuthService interface {
	Register(ctx context.Context, request dto.RegisterRequest) (dto.UserResponse, error)
	Login(ctx context.Context, request dto.LoginRequest) (dto.LoginResponse, error)
	Me(ctx context.Context, userID int64) (dto.UserResponse, error)
}

type authService struct {
	users  repository.UserRepository
	tokens *utils.TokenManager
	// inviteCode gates registration; empty means registration is open.
	inviteCode string
	// inviteCodeHash is a bcrypt hash of the same code and wins when both are set.
	inviteCodeHash string
}

func NewAuthService(
	users repository.UserRepository,
	tokens *utils.TokenManager,
	inviteCode string,
	inviteCodeHash string,
) AuthService {
	return &authService{
		users:          users,
		tokens:         tokens,
		inviteCode:     inviteCode,
		inviteCodeHash: inviteCodeHash,
	}
}

// errInvalidInvite is one message for every failure: a missing, wrong or expired-looking code
// all read the same, so the response reveals nothing about the code itself.
var errInvalidInvite = utils.NewFieldError("Validation failed", map[string]string{
	"invite_code": "invite code is not valid",
})

// checkInvite gates registration when an invite code is configured, by hash when one is
// available and by the plaintext otherwise. Both comparisons take the same time whatever the
// input, so a wrong code cannot be narrowed down one character at a time.
func (s *authService) checkInvite(provided string) error {
	code := strings.TrimSpace(provided)

	switch {
	case s.inviteCodeHash != "":
		// bcrypt compares in constant time for a given hash and is deliberately slow, which
		// also blunts guessing on top of the rate limit in front of this endpoint.
		if bcrypt.CompareHashAndPassword([]byte(s.inviteCodeHash), []byte(code)) != nil {
			return errInvalidInvite
		}
	case s.inviteCode != "":
		if subtle.ConstantTimeCompare([]byte(code), []byte(s.inviteCode)) != 1 {
			return errInvalidInvite
		}
	}
	return nil
}

func (s *authService) Register(ctx context.Context, request dto.RegisterRequest) (dto.UserResponse, error) {
	// Checked before anything else, so a wrong code cannot be used to probe which emails are
	// already registered.
	if err := s.checkInvite(request.InviteCode); err != nil {
		return dto.UserResponse{}, err
	}

	email := normalizeEmail(request.Email)

	exists, err := s.users.ExistsByEmail(ctx, email)
	if err != nil {
		return dto.UserResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to verify email availability", err)
	}
	if exists {
		return dto.UserResponse{}, utils.Conflict("Email is already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcryptCost)
	if err != nil {
		return dto.UserResponse{}, utils.WrapDomainError(utils.ErrValidation, "Failed to hash password", err)
	}

	now := time.Now().UTC()
	user := &model.User{
		Name:         strings.TrimSpace(request.Name),
		Email:        email,
		PasswordHash: string(hash),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return dto.UserResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to create user", err)
	}
	return toUserResponse(*user), nil
}

func (s *authService) Login(ctx context.Context, request dto.LoginRequest) (dto.LoginResponse, error) {
	user, err := s.users.FindByEmail(ctx, normalizeEmail(request.Email))
	if err != nil {
		return dto.LoginResponse{}, utils.WrapDomainError(utils.ErrUnauthorized, "Failed to verify credentials", err)
	}

	// The same message covers an unknown email and a wrong password, so the endpoint
	// cannot be used to enumerate registered addresses.
	const invalid = "Invalid email or password"
	if user == nil {
		return dto.LoginResponse{}, utils.Unauthorized(invalid)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)); err != nil {
		return dto.LoginResponse{}, utils.Unauthorized(invalid)
	}
	if !user.IsActive {
		return dto.LoginResponse{}, utils.Unauthorized("Account is inactive")
	}

	token, expiresAt, err := s.tokens.Generate(user.ID, user.Email)
	if err != nil {
		return dto.LoginResponse{}, utils.WrapDomainError(utils.ErrUnauthorized, "Failed to issue token", err)
	}

	return dto.LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User:        toUserResponse(*user),
	}, nil
}

func (s *authService) Me(ctx context.Context, userID int64) (dto.UserResponse, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return dto.UserResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load user", err)
	}
	if user == nil {
		return dto.UserResponse{}, utils.NotFound("User")
	}
	return toUserResponse(*user), nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func toUserResponse(user model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
	}
}
