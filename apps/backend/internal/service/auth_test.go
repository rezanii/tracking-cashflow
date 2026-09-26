package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

func newAuthService(t *testing.T) (AuthService, *fakeUserRepository, *utils.TokenManager) {
	t.Helper()
	users := newFakeUserRepository()
	tokens := utils.NewTokenManager("unit-test-secret-value-long-enough", time.Hour)
	return NewAuthService(users, tokens, "", ""), users, tokens
}

func TestRegisterStoresOnlyAHash(t *testing.T) {
	service, users, _ := newAuthService(t)

	created, err := service.Register(context.Background(), dto.RegisterRequest{
		Name:     "  John Doe  ",
		Email:    "  User@Example.com ",
		Password: "Admin123!",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if created.Name != "John Doe" {
		t.Fatalf("name = %q, want the trimmed John Doe", created.Name)
	}
	// The email is normalised so a later login with different casing still resolves.
	if created.Email != "user@example.com" {
		t.Fatalf("email = %q, want the lower cased user@example.com", created.Email)
	}
	if !created.IsActive {
		t.Fatal("a new user should be active")
	}

	stored := users.users["user@example.com"]
	if stored == nil {
		t.Fatal("user was not stored under the normalised email")
	}
	if stored.PasswordHash == "Admin123!" {
		t.Fatal("the password was stored in plain text")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("Admin123!")); err != nil {
		t.Fatalf("stored hash does not verify against the password: %v", err)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	service, _, _ := newAuthService(t)
	ctx := context.Background()

	request := dto.RegisterRequest{Name: "John", Email: "user@example.com", Password: "Admin123!"}
	if _, err := service.Register(ctx, request); err != nil {
		t.Fatalf("first Register returned error: %v", err)
	}

	// Casing must not be a way around the unique email.
	request.Email = "USER@example.com"
	_, err := service.Register(ctx, request)
	if err == nil {
		t.Fatal("registering a duplicate email should fail")
	}
	if !errors.Is(err, utils.ErrConflict) {
		t.Fatalf("error kind = %v, want ErrConflict", err)
	}
}

func TestLoginReturnsUsableToken(t *testing.T) {
	service, _, tokens := newAuthService(t)
	ctx := context.Background()

	created, err := service.Register(ctx, dto.RegisterRequest{Name: "John", Email: "user@example.com", Password: "Admin123!"})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	result, err := service.Login(ctx, dto.LoginRequest{Email: "user@example.com", Password: "Admin123!"})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if result.TokenType != "Bearer" {
		t.Fatalf("token type = %q, want Bearer", result.TokenType)
	}
	if result.User.ID != created.ID {
		t.Fatalf("user id = %d, want %d", result.User.ID, created.ID)
	}

	claims, err := tokens.Parse(result.AccessToken)
	if err != nil {
		t.Fatalf("the issued token does not parse: %v", err)
	}
	userID, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID returned error: %v", err)
	}
	if userID != created.ID {
		t.Fatalf("token subject = %d, want %d", userID, created.ID)
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	service, _, _ := newAuthService(t)
	ctx := context.Background()

	if _, err := service.Register(ctx, dto.RegisterRequest{Name: "John", Email: "user@example.com", Password: "Admin123!"}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	_, wrongPassword := service.Login(ctx, dto.LoginRequest{Email: "user@example.com", Password: "WrongPass1"})
	_, unknownEmail := service.Login(ctx, dto.LoginRequest{Email: "nobody@example.com", Password: "Admin123!"})

	if wrongPassword == nil || unknownEmail == nil {
		t.Fatal("both a wrong password and an unknown email must fail")
	}
	if !errors.Is(wrongPassword, utils.ErrUnauthorized) || !errors.Is(unknownEmail, utils.ErrUnauthorized) {
		t.Fatal("both failures must be unauthorized")
	}
	// Identical messages keep the endpoint from confirming which emails exist.
	if wrongPassword.Error() != unknownEmail.Error() {
		t.Fatalf("messages differ and leak account existence: %q vs %q", wrongPassword, unknownEmail)
	}
}

func TestLoginRejectsInactiveUser(t *testing.T) {
	service, users, _ := newAuthService(t)
	ctx := context.Background()

	if _, err := service.Register(ctx, dto.RegisterRequest{Name: "John", Email: "user@example.com", Password: "Admin123!"}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	users.users["user@example.com"].IsActive = false

	_, err := service.Login(ctx, dto.LoginRequest{Email: "user@example.com", Password: "Admin123!"})
	if err == nil {
		t.Fatal("an inactive user must not be able to log in")
	}
	if !errors.Is(err, utils.ErrUnauthorized) {
		t.Fatalf("error kind = %v, want ErrUnauthorized", err)
	}
}

func TestMeReturnsTheUser(t *testing.T) {
	service, _, _ := newAuthService(t)
	ctx := context.Background()

	created, err := service.Register(ctx, dto.RegisterRequest{Name: "John", Email: "user@example.com", Password: "Admin123!"})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	found, err := service.Me(ctx, created.ID)
	if err != nil {
		t.Fatalf("Me returned error: %v", err)
	}
	if found.Email != "user@example.com" {
		t.Fatalf("email = %q, want user@example.com", found.Email)
	}

	if _, err := service.Me(ctx, created.ID+999); !errors.Is(err, utils.ErrNotFound) {
		t.Fatalf("Me for an unknown id = %v, want ErrNotFound", err)
	}
}

func TestLoginSurfacesRepositoryFailureAsUnauthorized(t *testing.T) {
	users := newFakeUserRepository()
	users.failOn = "FindByEmail"
	service := NewAuthService(users, utils.NewTokenManager("unit-test-secret-value-long-enough", time.Hour), "", "")

	_, err := service.Login(context.Background(), dto.LoginRequest{Email: "user@example.com", Password: "Admin123!"})
	if err == nil {
		t.Fatal("a repository failure must not look like a success")
	}
	// The driver error stays wrapped so the handler cannot leak it to the client.
	domainErr, ok := utils.AsDomainError(err)
	if !ok {
		t.Fatalf("error %v is not a domain error", err)
	}
	if !errors.Is(domainErr.Cause(), errBoom) {
		t.Fatalf("cause = %v, want the repository failure", domainErr.Cause())
	}
}

// With an invite code configured, registration is closed to anyone without it. The check runs
// before the email lookup, so a wrong code cannot be used to discover which emails exist.
func TestRegisterRequiresTheInviteCodeWhenConfigured(t *testing.T) {
	users := newFakeUserRepository()
	service := NewAuthService(users, utils.NewTokenManager("0123456789012345678901234567890123", time.Hour), "undangan-rahasia", "")

	valid := dto.RegisterRequest{Name: "Orang Baru", Email: "baru@example.com", Password: "Passw0rd!"}

	for name, code := range map[string]string{
		"missing": "",
		"wrong":   "undangan-salah",
		"prefix":  "undangan",
	} {
		t.Run(name, func(t *testing.T) {
			request := valid
			request.InviteCode = code

			_, err := service.Register(context.Background(), request)
			if err == nil {
				t.Fatalf("registration succeeded with a %s invite code", name)
			}
			domainErr, ok := utils.AsDomainError(err)
			if !ok || domainErr.Kind != utils.ErrValidation {
				t.Fatalf("error = %v, want a validation error", err)
			}
			if _, named := domainErr.Fields["invite_code"]; !named {
				t.Fatalf("fields = %v, want the error on invite_code", domainErr.Fields)
			}
			if len(users.users) != 0 {
				t.Fatal("a user was created despite the invalid code")
			}
		})
	}

	t.Run("correct code, and surrounding space is tolerated", func(t *testing.T) {
		request := valid
		request.InviteCode = "  undangan-rahasia  "

		created, err := service.Register(context.Background(), request)
		if err != nil {
			t.Fatalf("Register returned error: %v", err)
		}
		if created.Email != "baru@example.com" {
			t.Fatalf("email = %q", created.Email)
		}
	})
}

// Without a configured code the endpoint stays open, which is what local development uses.
func TestRegisterStaysOpenWithoutAnInviteCode(t *testing.T) {
	users := newFakeUserRepository()
	service := NewAuthService(users, utils.NewTokenManager("0123456789012345678901234567890123", time.Hour), "", "")

	if _, err := service.Register(context.Background(), dto.RegisterRequest{
		Name: "Orang Baru", Email: "baru@example.com", Password: "Passw0rd!",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
}

// A hashed code is what a deployed instance should carry: whoever reads the environment
// learns the hash, and a bcrypt hash cannot be turned back into the code.
func TestRegisterAcceptsAHashedInviteCode(t *testing.T) {
	const code = "kode-otorisasi-yang-panjang"

	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
	if err != nil {
		t.Fatalf("hash invite code: %v", err)
	}

	users := newFakeUserRepository()
	tokens := utils.NewTokenManager("0123456789012345678901234567890123", time.Hour)
	// The plaintext is deliberately wrong here: the hash has to win over it.
	service := NewAuthService(users, tokens, "kode-plaintext-yang-salah", string(hash))

	valid := dto.RegisterRequest{Name: "Diundang", Email: "diundang@example.com", Password: "Passw0rd!"}

	t.Run("wrong code refused", func(t *testing.T) {
		request := valid
		request.InviteCode = "kode-plaintext-yang-salah"

		if _, err := service.Register(context.Background(), request); err == nil {
			t.Fatal("the plaintext was accepted although a hash is configured")
		}
		if len(users.users) != 0 {
			t.Fatal("a user was created")
		}
	})

	t.Run("correct code accepted", func(t *testing.T) {
		request := valid
		request.InviteCode = code

		if _, err := service.Register(context.Background(), request); err != nil {
			t.Fatalf("Register returned error: %v", err)
		}
	})
}

// Every rejection must read the same, so the response cannot be used to learn anything about
// the configured code.
func TestInviteRejectionIsIndistinguishable(t *testing.T) {
	users := newFakeUserRepository()
	tokens := utils.NewTokenManager("0123456789012345678901234567890123", time.Hour)
	service := NewAuthService(users, tokens, "kode-benar", "")

	messages := map[string]bool{}
	for _, code := range []string{"", "k", "kode-bena", "kode-benarX", "sama sekali lain"} {
		_, err := service.Register(context.Background(), dto.RegisterRequest{
			Name: "Uji", Email: "uji@example.com", Password: "Passw0rd!", InviteCode: code,
		})
		domainErr, ok := utils.AsDomainError(err)
		if !ok {
			t.Fatalf("code %q produced %v, want a domain error", code, err)
		}
		messages[domainErr.Message+"|"+domainErr.Fields["invite_code"]] = true
	}
	if len(messages) != 1 {
		t.Fatalf("rejections differ between inputs: %v", messages)
	}
}
