package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domainSession "github.com/execution-hub/execution-hub/internal/domain/session"
	domainUser "github.com/execution-hub/execution-hub/internal/domain/user"
)

// Service handles authentication.
type Service struct {
	userRepo    domainUser.Repository
	sessionRepo domainSession.Repository
	sessionTTL  time.Duration
	logger      zerolog.Logger
}

// NewService creates an auth service.
func NewService(userRepo domainUser.Repository, sessionRepo domainSession.Repository, sessionTTL time.Duration, logger zerolog.Logger) *Service {
	return &Service{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		sessionTTL:  sessionTTL,
		logger:      logger.With().Str("service", "auth").Logger(),
	}
}

// LoginResult contains login response.
type LoginResult struct {
	User    *domainUser.User
	Session *domainSession.Session
	Token   string
}

// Login authenticates a user and creates a session.
func (s *Service) Login(ctx context.Context, username, password string, userAgent, ipAddress *string) (*LoginResult, error) {
	username = domainUser.NormalizeUsername(username)
	u, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("invalid username or password")
	}
	if !u.IsActive() {
		return nil, fmt.Errorf("user is disabled")
	}
	if !domainUser.VerifyPassword(u.PasswordHash, password) {
		return nil, fmt.Errorf("invalid username or password")
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	tokenHash := hashToken(token)

	now := time.Now().UTC()
	sess := &domainSession.Session{
		SessionID:  uuid.New(),
		TokenHash:  tokenHash,
		UserID:     u.UserID,
		CreatedAt:  now,
		ExpiresAt:  now.Add(s.sessionTTL),
		LastSeenAt: &now,
		UserAgent:  userAgent,
		IPAddress:  ipAddress,
	}
	if err := s.sessionRepo.Create(ctx, sess); err != nil {
		return nil, err
	}

	s.logger.Info().Str("user_id", u.UserID.String()).Msg("user login")
	return &LoginResult{User: u, Session: sess, Token: token}, nil
}

// Authenticate validates a session token and returns the user.
func (s *Service) Authenticate(ctx context.Context, token string) (*domainUser.User, *domainSession.Session, error) {
	if token == "" {
		return nil, nil, fmt.Errorf("missing token")
	}
	tokenHash := hashToken(token)
	sess, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, nil, err
	}
	if sess == nil {
		return nil, nil, fmt.Errorf("session not found")
	}
	if sess.IsExpired(time.Now().UTC()) {
		_ = s.sessionRepo.DeleteByID(ctx, sess.SessionID)
		return nil, nil, fmt.Errorf("session expired")
	}
	u, err := s.userRepo.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	if u == nil || !u.IsActive() {
		return nil, nil, fmt.Errorf("user not active")
	}
	_ = s.sessionRepo.UpdateLastSeen(ctx, sess.SessionID)
	return u, sess, nil
}

// Logout deletes a session token.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessionRepo.DeleteByTokenHash(ctx, hashToken(token))
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
