package services

import (
	"context"
	"errors"
	"strings"
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
	ErrCustomerExists             = errors.New("an account with this email already exists")
	ErrCustomerInvalidCredentials = errors.New("invalid email or password")
	ErrCustomerInvalidSession     = errors.New("invalid customer session")
)

type CustomerService struct {
	cfg       config.CustomerConfig
	customers repositories.CustomerRepository
	sessions  repositories.CustomerSessionRepository
	throttle  *loginThrottle
}

func NewCustomerService(cfg config.CustomerConfig, customers repositories.CustomerRepository, sessions repositories.CustomerSessionRepository) *CustomerService {
	return &CustomerService{
		cfg:       cfg,
		customers: customers,
		sessions:  sessions,
		throttle: &loginThrottle{
			failures:    make(map[string]int),
			lockedUntil: make(map[string]time.Time),
		},
	}
}

func (s *CustomerService) Signup(ctx context.Context, email, password, remoteAddr string) (models.Customer, string, string, time.Time, error) {
	email = normalizeEmail(email)
	if email == "" || len(password) < 12 || len(password) > 72 {
		return models.Customer{}, "", "", time.Time{}, ErrCustomerInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return models.Customer{}, "", "", time.Time{}, err
	}
	now := time.Now()
	customer := models.Customer{
		ID:           primitive.NewObjectID(),
		Email:        email,
		PasswordHash: string(hash),
		Status:       models.CustomerStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	id, err := s.customers.Insert(ctx, customer)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Customer{}, "", "", time.Time{}, ErrCustomerExists
		}
		return models.Customer{}, "", "", time.Time{}, err
	}
	if oid, err := primitive.ObjectIDFromHex(id); err == nil {
		customer.ID = oid
	}
	return s.createSession(ctx, customer)
}

func (s *CustomerService) Login(ctx context.Context, email, password, remoteAddr string) (models.Customer, string, string, time.Time, error) {
	email = normalizeEmail(email)
	if s.throttle.isLocked(remoteAddr, email) {
		return models.Customer{}, "", "", time.Time{}, ErrTooManyAttempts
	}
	customer, err := s.customers.FindByEmail(ctx, email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(customer.PasswordHash), []byte(password)) != nil || customer.Status != models.CustomerStatusActive {
		s.throttle.recordFailure(remoteAddr, email)
		return models.Customer{}, "", "", time.Time{}, ErrCustomerInvalidCredentials
	}
	s.throttle.clear(remoteAddr, email)
	return s.createSession(ctx, customer)
}

func (s *CustomerService) Authenticate(ctx context.Context, token string) (models.Customer, models.CustomerSession, error) {
	if token == "" {
		return models.Customer{}, models.CustomerSession{}, ErrCustomerInvalidSession
	}
	session, err := s.sessions.FindByTokenHash(ctx, identity.HashToken(token))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Customer{}, models.CustomerSession{}, ErrCustomerInvalidSession
		}
		return models.Customer{}, models.CustomerSession{}, err
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.sessions.DeleteByTokenHash(ctx, session.TokenHash)
		return models.Customer{}, models.CustomerSession{}, ErrCustomerInvalidSession
	}
	customer, err := s.customers.FindByID(ctx, session.CustomerID.Hex())
	if err != nil || customer.Status != models.CustomerStatusActive {
		return models.Customer{}, models.CustomerSession{}, ErrCustomerInvalidSession
	}
	return customer, session, nil
}

func (s *CustomerService) ValidateCSRF(session models.CustomerSession, csrf string) bool {
	return csrf != "" && identity.HashToken(csrf) == session.CSRFHash
}

func (s *CustomerService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessions.DeleteByTokenHash(ctx, identity.HashToken(token))
}

func (s *CustomerService) SessionTTL() time.Duration {
	return s.cfg.SessionTTL
}

func (s *CustomerService) SecureCookies() bool {
	return s.cfg.SecureCookies
}

func (s *CustomerService) createSession(ctx context.Context, customer models.Customer) (models.Customer, string, string, time.Time, error) {
	token, err := randomToken(32)
	if err != nil {
		return models.Customer{}, "", "", time.Time{}, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return models.Customer{}, "", "", time.Time{}, err
	}
	now := time.Now()
	session := models.CustomerSession{
		ID:         primitive.NewObjectID(),
		CustomerID: customer.ID,
		TokenHash:  identity.HashToken(token),
		CSRFHash:   identity.HashToken(csrf),
		ExpiresAt:  now.Add(s.cfg.SessionTTL),
		CreatedAt:  now,
	}
	if err := s.sessions.Insert(ctx, session); err != nil {
		return models.Customer{}, "", "", time.Time{}, err
	}
	return customer, token, csrf, session.ExpiresAt, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
