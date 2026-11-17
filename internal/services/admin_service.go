package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/identity"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrTooManyAttempts    = errors.New("too many login attempts")
	ErrInvalidSession     = errors.New("invalid admin session")
	ErrInvalidCSRF        = errors.New("invalid csrf token")
)

type loginThrottle struct {
	mu          sync.Mutex
	failures    map[string]int
	lockedUntil map[string]time.Time
}

type AdminService struct {
	cfg      config.AdminConfig
	sessions repositories.AdminSessionRepository
	throttle *loginThrottle
}

func NewAdminService(cfg config.AdminConfig, sessions repositories.AdminSessionRepository) *AdminService {
	return &AdminService{
		cfg:      cfg,
		sessions: sessions,
		throttle: &loginThrottle{
			failures:    make(map[string]int),
			lockedUntil: make(map[string]time.Time),
		},
	}
}

func (s *AdminService) Login(ctx context.Context, username, password, remoteAddr string) (string, string, time.Time, error) {
	if s.throttle.isLocked(remoteAddr, username) {
		return "", "", time.Time{}, ErrTooManyAttempts
	}
	if username != s.cfg.Username || bcrypt.CompareHashAndPassword([]byte(s.cfg.PasswordHash), []byte(password)) != nil {
		s.throttle.recordFailure(remoteAddr, username)
		return "", "", time.Time{}, ErrInvalidCredentials
	}
	s.throttle.clear(remoteAddr, username)

	token, err := randomToken(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	now := time.Now()
	session := models.AdminSession{
		ID:        primitiveObjectID(),
		TokenHash: identity.HashToken(token),
		CSRFHash:  identity.HashToken(csrf),
		ExpiresAt: now.Add(s.cfg.SessionTTL),
		CreatedAt: now,
	}
	if err := s.sessions.Insert(ctx, session); err != nil {
		return "", "", time.Time{}, err
	}
	return token, csrf, session.ExpiresAt, nil
}

func (s *AdminService) Authenticate(ctx context.Context, token string) (models.AdminSession, error) {
	if token == "" {
		return models.AdminSession{}, ErrInvalidSession
	}
	session, err := s.sessions.FindByTokenHash(ctx, identity.HashToken(token))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.AdminSession{}, ErrInvalidSession
		}
		return models.AdminSession{}, err
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.sessions.DeleteByTokenHash(ctx, session.TokenHash)
		return models.AdminSession{}, ErrInvalidSession
	}
	return session, nil
}

func (s *AdminService) ValidateCSRF(session models.AdminSession, csrf string) bool {
	return csrf != "" && identity.HashToken(csrf) == session.CSRFHash
}

func (s *AdminService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessions.DeleteByTokenHash(ctx, identity.HashToken(token))
}

func (s *AdminService) Username() string {
	return s.cfg.Username
}

func (s *AdminService) SecureCookies() bool {
	return s.cfg.SecureCookies
}

func (s *AdminService) SessionTTL() time.Duration {
	return s.cfg.SessionTTL
}

func (t *loginThrottle) isLocked(ip, username string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := ip + "|" + username
	lockedUntil, ok := t.lockedUntil[key]
	return ok && time.Now().Before(lockedUntil)
}

func (t *loginThrottle) recordFailure(ip, username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := ip + "|" + username
	t.failures[key]++
	if t.failures[key] >= 5 {
		t.lockedUntil[key] = time.Now().Add(5 * time.Minute)
		t.failures[key] = 0
	}
}

func (t *loginThrottle) clear(ip, username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := ip + "|" + username
	delete(t.failures, key)
	delete(t.lockedUntil, key)
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func primitiveObjectID() primitive.ObjectID {
	return primitive.NewObjectID()
}
