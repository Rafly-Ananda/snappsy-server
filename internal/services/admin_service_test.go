package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type fakeAdminSessionRepository struct {
	sessions map[string]models.AdminSession
}

func (f *fakeAdminSessionRepository) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeAdminSessionRepository) Insert(ctx context.Context, session models.AdminSession) error {
	if f.sessions == nil {
		f.sessions = make(map[string]models.AdminSession)
	}
	f.sessions[session.TokenHash] = session
	return nil
}
func (f *fakeAdminSessionRepository) FindByTokenHash(ctx context.Context, tokenHash string) (models.AdminSession, error) {
	session, ok := f.sessions[tokenHash]
	if !ok {
		return models.AdminSession{}, errors.New("not found")
	}
	return session, nil
}
func (f *fakeAdminSessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	delete(f.sessions, tokenHash)
	return nil
}

func TestAdminLoginAuthenticatesAndValidatesCSRF(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	repo := &fakeAdminSessionRepository{}
	svc := NewAdminService(config.AdminConfig{
		Username:     "operator",
		PasswordHash: string(hash),
		SessionTTL:   time.Hour,
	}, repo)

	token, csrf, _, err := svc.Login(context.Background(), "operator", "secret-password", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token == "" || csrf == "" {
		t.Fatalf("Login() returned empty token or csrf")
	}
	session, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if !svc.ValidateCSRF(session, csrf) {
		t.Fatal("ValidateCSRF() = false for issued token")
	}
	if svc.ValidateCSRF(session, "wrong-csrf") {
		t.Fatal("ValidateCSRF() = true for wrong token")
	}
}

func TestAdminLoginRejectsBadPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	svc := NewAdminService(config.AdminConfig{
		Username:     "operator",
		PasswordHash: string(hash),
		SessionTTL:   time.Hour,
	}, &fakeAdminSessionRepository{})

	_, _, _, err = svc.Login(context.Background(), "operator", "wrong-password", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v", err)
	}
}
