package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/template/internal/model"
	"github.com/ssr0016/template/internal/repository"
)

const testPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

func TestAuthService_Register_Success(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil)

	user, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "test@example.com",
		Name:     "Test User",
		Password: testPassword,
	})

	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}
	if user == nil {
		t.Fatal("Register() returned nil user")
	}
	if user.Email != "test@example.com" {
		t.Errorf("Email = %s, want test@example.com", user.Email)
	}
	if user.Name != "Test User" {
		t.Errorf("Name = %s, want Test User", user.Name)
	}
	if user.PasswordHash == "" {
		t.Error("PasswordHash is empty")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(testPassword)); err != nil {
		t.Errorf("Password hash doesn't match: %v", err)
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil)

	_, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "dup@example.com",
		Name:     "User 1",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("First Register() failed: %v", err)
	}

	_, err = svc.Register(context.Background(), model.RegisterRequest{
		Email:    "dup@example.com",
		Name:     "User 2",
		Password: testPassword,
	})

	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("Register() error = %v, want ErrEmailTaken", err)
	}
}

func TestAuthService_Register_RepoError(t *testing.T) {
	repo := repository.NewMockUserRepo()
	repo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("db error")
	}
	svc := NewAuthService(repo, nil)

	_, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "err@example.com",
		Name:     "Err User",
		Password: testPassword,
	})

	if err == nil {
		t.Fatal("Register() expected error, got nil")
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil)

	registered, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "login@example.com",
		Name:     "Login User",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	user, err := svc.Login(context.Background(), "login@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() failed: %v", err)
	}
	if user.ID != registered.ID {
		t.Errorf("User.ID = %d, want %d", user.ID, registered.ID)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil)

	_, err := svc.Login(context.Background(), "nobody@example.com", testPassword)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil)

	_, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "wrong@example.com",
		Name:     "Wrong User",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	_, err = svc.Login(context.Background(), "wrong@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_RepoError(t *testing.T) {
	repo := repository.NewMockUserRepo()
	repo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("db error")
	}
	svc := NewAuthService(repo, nil)

	_, err := svc.Login(context.Background(), "err@example.com", testPassword)
	if err == nil {
		t.Fatal("Login() expected error, got nil")
	}
}
