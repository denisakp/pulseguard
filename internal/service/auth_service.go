package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultPassword is the initial password for new accounts
	DefaultPassword = "password"
	// BCryptCost is the cost factor for bcrypt hashing
	BCryptCost = 12
	// OTPLength is the length of the OTP code
	OTPLength = 6
	// BackupCodeCount is the number of backup codes to generate
	BackupCodeCount = 10
)

// AuthService handles authentication logic with database persistence
type AuthService struct {
	userRepo       port.UserRepository
	jwtManager     *JWTManager
	sessionService *SessionService
	legacyEmail    string
	legacyPassword string
}

// NewAuthService creates a new authentication service with database support
func NewAuthService(userRepo port.UserRepository, jwtManager *JWTManager) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

// SetSessionService wires the session service so login flows can issue a
// session row + bind the JWT to it via the sid claim.
func (s *AuthService) SetSessionService(svc *SessionService) {
	s.sessionService = svc
}

// LoginContext carries request metadata used to attach a session to a token.
type LoginContext struct {
	UserAgent string
	IP        string
}

// NewLegacyAuthService creates a service with hardcoded credentials (deprecated, for backward compat)
func NewLegacyAuthService(email, password, jwtSecret string) *AuthService {
	jwtManager := NewJWTManager(jwtSecret, "ogoune", 24*time.Hour)
	return &AuthService{
		legacyEmail:    email,
		legacyPassword: password,
		jwtManager:     jwtManager,
	}
}

// Login validates credentials and returns a login response (may require 2FA)
func (s *AuthService) Login(ctx context.Context, email, password string) (*dto.LoginResponse, error) {
	resp := &dto.LoginResponse{
		Email: email,
	}

	// Trim input
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	// Try to find user in database
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Try legacy hardcoded auth for backward compatibility
			if s.legacyEmail != "" && email == s.legacyEmail && password == s.legacyPassword {
				// Generate or fetch default user
				return s.handleLegacyLogin(ctx, email)
			}
			return resp, ErrInvalidCredentials
		}
		return resp, err
	}

	// Check if password is initialized
	if !user.PasswordInitialized {
		// User must use default password on first login
		if password != DefaultPassword {
			return resp, ErrInvalidCredentials
		}
		// Password is correct, but user must initialize password on next step
		resp.Requires2FA = false
		resp.ForcePasswordChange = true
		resp.PasswordInitialized = false
		return resp, nil
	}

	// Verify password hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password)); err != nil {
		return resp, ErrInvalidCredentials
	}

	// Check if 2FA is enabled
	if user.TwoFactorEnabled {
		// Generate temporary session for 2FA
		sessionID := generateRandomString(32)
		resp.Requires2FA = true
		resp.TwoFactorSessionID = sessionID
		resp.ForcePasswordChange = user.ForcePasswordChange
		resp.PasswordInitialized = user.PasswordInitialized
		// Store session temporarily (in production, use Redis or similar)
		// For now, we'll validate directly in Verify2FA
		return resp, nil
	}

	// Generate JWT token, optionally bound to a session row.
	token, err := s.issueTokenWithSession(ctx, email, user.ID)
	if err != nil {
		return resp, err
	}

	// Update last login
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	resp.Token = token
	resp.ForcePasswordChange = user.ForcePasswordChange
	resp.PasswordInitialized = user.PasswordInitialized
	return resp, nil
}

// issueTokenWithSession issues a session (when wired) + a JWT bound to it.
// Falls back to a sessionless token when session service is unavailable.
func (s *AuthService) issueTokenWithSession(ctx context.Context, email, userID string) (string, error) {
	if s.sessionService == nil {
		slog.Warn("auth: session service unwired — issuing token without sid",
			"user_id", userID,
		)
		return s.jwtManager.Generate(ctx, email, userID)
	}
	meta, _ := ctx.Value(loginCtxKey{}).(LoginContext)
	sess, err := s.sessionService.Issue(ctx, userID, meta.UserAgent, meta.IP)
	if err != nil {
		// Don't block login on session insert failures — fall back to legacy.
		slog.Warn("auth: session insert failed — issuing token without sid",
			"user_id", userID,
			"error", err,
		)
		return s.jwtManager.Generate(ctx, email, userID)
	}
	slog.Info("auth: session issued",
		"user_id", userID,
		"session_id", sess.ID,
		"browser", sess.Browser,
		"os", sess.OS,
	)
	return s.jwtManager.GenerateWithSession(ctx, email, userID, sess.ID)
}

type loginCtxKey struct{}

// WithLoginContext attaches request metadata so login flows can use it without
// changing public signatures everywhere.
func WithLoginContext(ctx context.Context, meta LoginContext) context.Context {
	return context.WithValue(ctx, loginCtxKey{}, meta)
}

// Verify2FA validates the OTP and issues JWT if valid
func (s *AuthService) Verify2FA(ctx context.Context, email, otp string) (string, error) {
	email = strings.TrimSpace(email)
	otp = strings.TrimSpace(otp)

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	if !user.TwoFactorEnabled || user.TwoFactorSecret == "" {
		return "", errors.New("2FA not enabled for this user")
	}

	// Verify OTP
	if !totp.Validate(otp, user.TwoFactorSecret) {
		return "", errors.New("invalid OTP")
	}

	// Generate JWT token bound to a fresh session row.
	token, err := s.issueTokenWithSession(ctx, email, user.ID)
	if err != nil {
		return "", err
	}

	// Update last login
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	return token, nil
}

// InitializePassword sets the user's custom password on first login
func (s *AuthService) InitializePassword(ctx context.Context, email, newPassword string) (*domain.User, error) {
	email = strings.TrimSpace(email)
	newPassword = strings.TrimSpace(newPassword)

	if len(newPassword) < 8 {
		return nil, ErrInvalidPassword
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrResourceNotFound
	}

	// Hash new password
	hash, err := hashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	// Update password
	if err := s.userRepo.UpdatePassword(ctx, user.ID, hash); err != nil {
		return nil, err
	}

	// Fetch updated user
	return s.userRepo.FindByID(ctx, user.ID)
}

// ChangePassword updates the user's password
func (s *AuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrInvalidPassword
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return ErrResourceNotFound
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	// Hash new password
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password
	return s.userRepo.UpdatePassword(ctx, userID, hash)
}

// ResetPasswordToDefault resets the password to the default value
func (s *AuthService) ResetPasswordToDefault(ctx context.Context, userID, currentPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return ErrResourceNotFound
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	// Hash default password
	hash, err := hashPassword(DefaultPassword)
	if err != nil {
		return err
	}

	// Update password and set force password change flag
	if err := s.userRepo.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}

	// Set force password change
	user.ForcePasswordChange = true
	return s.userRepo.Update(ctx, user)
}

// GenerateTOTPSecret generates a TOTP secret for 2FA setup
func (s *AuthService) GenerateTOTPSecret(ctx context.Context, userID, userEmail string) (*dto.Enable2FAResponse, error) {
	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Ogoune",
		AccountName: userEmail,
		SecretSize:  32,
	})
	if err != nil {
		return nil, err
	}

	// Get the OTPAuth URI for QR code generation on frontend
	// Frontend will use qrcode.js or similar library to generate the QR code
	otpAuthURI := key.URL()

	// Generate backup codes
	backupCodes := generateBackupCodes(BackupCodeCount)

	return &dto.Enable2FAResponse{
		Secret:      key.Secret(),
		QRCode:      otpAuthURI, // Send the otpauth:// URI to frontend for QR code generation
		BackupCodes: backupCodes,
	}, nil
}

// Enable2FA enables 2FA for a user after OTP verification
func (s *AuthService) Enable2FA(ctx context.Context, userID, secret, otp string) error {
	// Verify OTP matches the secret
	if !totp.Validate(otp, secret) {
		return errors.New("invalid OTP")
	}

	// Update user 2FA secret and enable flag
	return s.userRepo.UpdateTwoFactorSecret(ctx, userID, secret, true)
}

// Disable2FA disables 2FA for a user
func (s *AuthService) Disable2FA(ctx context.Context, userID, password string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return ErrResourceNotFound
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	// Disable 2FA
	return s.userRepo.UpdateTwoFactorSecret(ctx, userID, "", false)
}

// IssueTokenForUser is the handler-facing wrapper around issueTokenWithSession.
// Used by flows like InitializePassword that auto-login after a non-credential
// state change. Honors WithLoginContext.
func (s *AuthService) IssueTokenForUser(ctx context.Context, email, userID string) (string, error) {
	return s.issueTokenWithSession(ctx, email, userID)
}

// ValidateToken validates a JWT token and returns the email and user ID.
func (s *AuthService) ValidateToken(tokenString string) (string, string, error) {
	claims, err := s.jwtManager.Validate(tokenString)
	if err != nil {
		return "", "", ErrInvalidToken
	}

	return claims.Email, claims.UserID, nil
}

// ValidateTokenFull is like ValidateToken but also returns the session ID
// embedded in the token (empty when the token predates session binding).
func (s *AuthService) ValidateTokenFull(tokenString string) (email, userID, sessionID string, err error) {
	claims, err := s.jwtManager.Validate(tokenString)
	if err != nil {
		return "", "", "", ErrInvalidToken
	}
	return claims.Email, claims.UserID, claims.SessionID, nil
}

// UpdateProfile updates user name and email
func (s *AuthService) UpdateProfile(ctx context.Context, userID, name, email string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)

	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrResourceNotFound
	}

	// Check if new email is unique (if changing)
	if email != user.Email {
		existing, _ := s.userRepo.FindByEmail(ctx, email)
		if existing != nil {
			return nil, errors.New("email already in use")
		}
	}

	user.Name = name
	user.Email = email

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (s *AuthService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

// GetUserByEmail fetches a user by email
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.userRepo.FindByEmail(ctx, strings.TrimSpace(email))
}

// CreateDefaultUser creates the default admin user if it doesn't exist (for first startup)
func (s *AuthService) CreateDefaultUser(ctx context.Context, email, password string) (*domain.User, error) {
	// Check if user already exists
	existing, _ := s.userRepo.FindByEmail(ctx, email)
	if existing != nil {
		return existing, nil
	}

	// Hash password
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:               email,
		Name:                "Administrator",
		HashedPassword:      hash,
		PasswordInitialized: true,
	}

	return s.userRepo.Create(ctx, user)
}

// handleLegacyLogin handles login with legacy hardcoded credentials
func (s *AuthService) handleLegacyLogin(ctx context.Context, email string) (*dto.LoginResponse, error) {
	// Try to find or create user with legacy credentials
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Create user with default password
			hash, _ := hashPassword(DefaultPassword)
			user = &domain.User{
				Email:               email,
				Name:                "Administrator",
				HashedPassword:      hash,
				PasswordInitialized: true,
			}
			user, _ = s.userRepo.Create(ctx, user)
		}
	}

	if user == nil {
		return &dto.LoginResponse{Email: email}, ErrInvalidCredentials
	}

	// Generate token bound to a session row when available.
	token, err := s.issueTokenWithSession(ctx, email, user.ID)
	if err != nil {
		return &dto.LoginResponse{Email: email}, err
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	return &dto.LoginResponse{
		Token:               token,
		Email:               email,
		PasswordInitialized: user.PasswordInitialized,
		ForcePasswordChange: user.ForcePasswordChange,
	}, nil
}

// hashPassword hashes a password using bcrypt
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BCryptCost)
	return string(hash), err
}

// generateRandomString generates a random base64-encoded string
func generateRandomString(length int) string {
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)[:length]
}

// generateBackupCodes generates backup codes for account recovery
func generateBackupCodes(count int) []string {
	codes := make([]string, count)
	for i := range count {
		codes[i] = generateRandomString(8)
	}
	return codes
}

// imageToDataURL converts an image to a data URL for QR codes
func imageToDataURL(_ any) string {
	// In production, this would encode the image properly
	// For now, return a placeholder
	return "data:image/png;base64,..."
}
