package service

import "errors"

// Service layer errors
var (
	// ErrValidationFailed indicates that input validation failed
	ErrValidationFailed = errors.New("validation failed")

	// ErrResourceNotFound indicates the requested resource was not found
	ErrResourceNotFound = errors.New("resource not found")

	// ErrSchedulerSync indicates scheduler synchronization failed
	ErrSchedulerSync = errors.New("scheduler synchronization failed")

	// ErrMaintenanceNotFound indicates the requested maintenance was not found
	ErrMaintenanceNotFound = errors.New("maintenance not found")

	// ErrInvalidCredentials is returned when email or password is incorrect
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrUnauthorized is returned when authentication is required but not provided
	ErrUnauthorized = errors.New("unauthorized: authentication required")

	// ErrInvalidToken is returned when JWT token is invalid or expired
	ErrInvalidToken = errors.New("invalid or expired token")

	// ErrInvalidPassword is returned when password doesn't meet requirements
	ErrInvalidPassword = errors.New("password must be at least 8 characters")

	// ErrAPIKeyNotFound indicates the requested API key doesn't exist for the user
	ErrAPIKeyNotFound = errors.New("api key not found")

	// ErrAPIKeyLimitReached indicates user reached the hard API key limit
	ErrAPIKeyLimitReached = errors.New("maximum number of API keys reached")

	// ErrAPIKeyExpired indicates an API key has passed its expiry date
	ErrAPIKeyExpired = errors.New("API key has expired")

	// ErrAPIKeyRevoked indicates an API key has been revoked
	ErrAPIKeyRevoked = errors.New("invalid or revoked API key")

	// ErrAPIKeyInvalid indicates API key lookup failed
	ErrAPIKeyInvalid = errors.New("invalid or revoked API key")

	// ErrICMPUnavailable is returned when an ICMP monitor cannot be created because ICMP
	// is disabled or the runtime capability is not available on this host.
	ErrICMPUnavailable = errors.New("ICMP monitoring is unavailable: enable ENABLE_ICMP and ensure raw socket capability")

	// ErrCredentialNotFound is returned when no credential exists for a resource.
	ErrCredentialNotFound = errors.New("credential not found")

	// ErrCredentialInvalid is returned when the supplied credential payload fails validation.
	ErrCredentialInvalid = errors.New("invalid credential payload")

	// Agent device monitoring (spec 079)

	// ErrHostNotFound is returned when a host does not exist.
	ErrHostNotFound = errors.New("host not found")

	// ErrHostCredentialInvalid is returned when an agent credential is missing,
	// malformed, unknown, or revoked.
	ErrHostCredentialInvalid = errors.New("invalid or revoked host credential")
)
