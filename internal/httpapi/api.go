package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/open-proofline/server/internal/coordination"
	"github.com/open-proofline/server/internal/email"
	"github.com/open-proofline/server/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultMaxUploadBytes        = int64(250 * 1024 * 1024)
	defaultAccountBlobQuotaBytes = int64(10 * 1024 * 1024 * 1024)
	defaultIncidentTokenTTL      = 24 * time.Hour
	defaultSessionTTL            = 12 * time.Hour
	defaultVerificationTTL       = 24 * time.Hour
	defaultSecondFactorEmailTTL  = 10 * time.Minute
	jsonBodyLimit                = int64(64 * 1024)
	fieldLimit                   = int64(64 * 1024)
	multipartOverhead            = int64(1024 * 1024)
	maxSafeUploadBytes           = int64(1<<63 - 1 - multipartOverhead)
)

// Options configures API construction.
type Options struct {
	MaxUploadBytes             int64
	AccountBlobQuotaBytes      int64
	DefaultIncidentTokenTTL    *time.Duration
	SessionTTL                 time.Duration
	BootstrapSecret            string
	WebAuth                    WebAuthConfig
	WebAuthn                   WebAuthnConfig
	AccountRegistration        AccountRegistrationConfig
	SecondFactorEmailTTL       time.Duration
	EmailSender                email.Sender
	MainRateLimit              MainRateLimitConfig
	MainRateLimiter            RateLimiter
	PublicRateLimit            PublicRateLimitConfig
	PublicRateLimiter          RateLimiter
	UploadCoordinator          coordination.Coordinator
	UploadCoordinationLeaseTTL time.Duration
	PasswordCost               int
	Logger                     *slog.Logger
}

// MainRateLimitConfig configures app-level limits for main API route classes.
type MainRateLimitConfig struct {
	Enabled            bool
	Window             time.Duration
	AuthLimit          int
	AuthRegisterLimit  int
	AuthEmailVerify    int
	BootstrapLimit     int
	AccountLimit       int
	IncidentReadLimit  int
	IncidentWriteLimit int
	UploadLimit        int
	ReconcileLimit     int
	StreamLimit        int
	TokenLimit         int
	DownloadLimit      int
	AdminLimit         int
}

// PublicRateLimitConfig configures app-level limits for public incident viewer
// route classes.
type PublicRateLimitConfig struct {
	Enabled       bool
	Window        time.Duration
	PageLimit     int
	DataLimit     int
	DownloadLimit int
	StaticLimit   int
}

// WebAuthConfig configures optional browser cookie-session authentication for
// the main API route tree.
type WebAuthConfig struct {
	Enabled               bool
	AllowedOrigins        []string
	SessionCookieName     string
	SessionCookieSecure   bool
	SessionCookieSameSite http.SameSite
	CSRFHeaderName        string
}

// WebAuthnConfig configures optional WebAuthn passkey/security-key
// second-factor support for the main API route tree.
type WebAuthnConfig struct {
	Enabled          bool
	RPID             string
	RPDisplayName    string
	AllowedOrigins   []string
	UserVerification string
	ChallengeTTL     time.Duration
}

// AccountRegistrationConfig controls unauthenticated account registration.
type AccountRegistrationConfig struct {
	Mode                 string
	EmailVerificationTTL time.Duration
	PublicWebOrigin      string
}

const (
	AccountRegistrationDisabled  = "disabled"
	AccountRegistrationAdminOnly = "admin_only"
	AccountRegistrationOpen      = "open"
	AccountRegistrationPaid      = "paid"
)

// RateLimiter records one request against a safe limiter key.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// PublicRateLimiter is kept as a compatibility name for the public viewer
// limiter interface.
type PublicRateLimiter = RateLimiter

// API holds the dependencies and limits used by the HTTP handlers.
type API struct {
	repo                       MetadataRepository
	store                      storage.BlobStore
	maxUploadBytes             int64
	accountBlobQuotaBytes      int64
	defaultIncidentTokenTTL    time.Duration
	sessionTTL                 time.Duration
	bootstrapSecret            string
	webAuth                    WebAuthConfig
	webAuthn                   WebAuthnConfig
	accountRegistration        AccountRegistrationConfig
	secondFactorEmailTTL       time.Duration
	emailSender                email.Sender
	mainRateLimit              MainRateLimitConfig
	mainRateLimiter            RateLimiter
	publicRateLimit            PublicRateLimitConfig
	publicRateLimiter          RateLimiter
	uploadCoordinator          coordination.Coordinator
	uploadCoordinationLeaseTTL time.Duration
	passwordCost               int
	logger                     *slog.Logger
}

// New builds the main API and incident viewer HTTP handler. Prefer NewMain or
// NewAdmin at call sites that need to make the routing boundary explicit.
func New(repo MetadataRepository, store storage.BlobStore, opts Options) http.Handler {
	return NewMain(repo, store, opts)
}

// NewMain builds the HTTP handler tree for the main API and read-only incident
// viewer listener.
func NewMain(repo MetadataRepository, store storage.BlobStore, opts Options) http.Handler {
	return newAPI(repo, store, opts).mainRoutes()
}

// NewAdmin builds the HTTP handler tree for the private admin dashboard
// listener.
func NewAdmin(repo MetadataRepository, store storage.BlobStore, opts Options) http.Handler {
	return newAPI(repo, store, opts).adminRoutes()
}

// NewPrivate builds the private admin dashboard listener handler tree. It is
// kept as a compatibility name for older internal callers.
func NewPrivate(repo MetadataRepository, store storage.BlobStore, opts Options) http.Handler {
	return NewAdmin(repo, store, opts)
}

// NewPublic builds the read-only incident viewer handler tree. The current
// server process mounts these routes on the main listener through NewMain.
func NewPublic(repo MetadataRepository, store storage.BlobStore, opts Options) http.Handler {
	return newAPI(repo, store, opts).publicRoutes()
}

func newAPI(repo MetadataRepository, store storage.BlobStore, opts Options) *API {
	maxUploadBytes := opts.MaxUploadBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	if maxUploadBytes > maxSafeUploadBytes {
		maxUploadBytes = maxSafeUploadBytes
	}
	accountBlobQuotaBytes := opts.AccountBlobQuotaBytes
	if accountBlobQuotaBytes <= 0 {
		accountBlobQuotaBytes = defaultAccountBlobQuotaBytes
	}
	incidentTokenTTL := defaultIncidentTokenTTL
	if opts.DefaultIncidentTokenTTL != nil {
		incidentTokenTTL = *opts.DefaultIncidentTokenTTL
	}
	if incidentTokenTTL < 0 {
		incidentTokenTTL = 0
	}
	sessionTTL := opts.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = defaultSessionTTL
	}
	passwordCost := opts.PasswordCost
	if passwordCost == 0 {
		passwordCost = bcrypt.DefaultCost
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	mainRateLimiter := opts.MainRateLimiter
	if opts.MainRateLimit.Enabled && mainRateLimiter == nil {
		mainRateLimiter = NewMemoryRateLimiter()
	}
	publicRateLimiter := opts.PublicRateLimiter
	if opts.PublicRateLimit.Enabled && publicRateLimiter == nil {
		publicRateLimiter = NewMemoryRateLimiter()
	}
	webAuth := opts.WebAuth
	if webAuth.SessionCookieName == "" {
		webAuth.SessionCookieName = "__Host-proofline_session"
	}
	if webAuth.SessionCookieSameSite == 0 {
		webAuth.SessionCookieSameSite = http.SameSiteLaxMode
	}
	if webAuth.CSRFHeaderName == "" {
		webAuth.CSRFHeaderName = "X-CSRF-Token"
	}
	webAuthn := opts.WebAuthn
	if webAuthn.RPDisplayName == "" {
		webAuthn.RPDisplayName = "Proofline"
	}
	if webAuthn.UserVerification == "" {
		webAuthn.UserVerification = "required"
	}
	if webAuthn.ChallengeTTL <= 0 {
		webAuthn.ChallengeTTL = 5 * time.Minute
	}
	accountRegistration := opts.AccountRegistration
	if accountRegistration.Mode == "" {
		accountRegistration.Mode = AccountRegistrationDisabled
	}
	if accountRegistration.EmailVerificationTTL <= 0 {
		accountRegistration.EmailVerificationTTL = defaultVerificationTTL
	}
	secondFactorEmailTTL := opts.SecondFactorEmailTTL
	if secondFactorEmailTTL <= 0 {
		secondFactorEmailTTL = defaultSecondFactorEmailTTL
	}

	return &API{
		repo:                       repo,
		store:                      store,
		maxUploadBytes:             maxUploadBytes,
		accountBlobQuotaBytes:      accountBlobQuotaBytes,
		defaultIncidentTokenTTL:    incidentTokenTTL,
		sessionTTL:                 sessionTTL,
		bootstrapSecret:            opts.BootstrapSecret,
		webAuth:                    webAuth,
		webAuthn:                   webAuthn,
		accountRegistration:        accountRegistration,
		secondFactorEmailTTL:       secondFactorEmailTTL,
		emailSender:                opts.EmailSender,
		mainRateLimit:              opts.MainRateLimit,
		mainRateLimiter:            mainRateLimiter,
		publicRateLimit:            opts.PublicRateLimit,
		publicRateLimiter:          publicRateLimiter,
		uploadCoordinator:          opts.UploadCoordinator,
		uploadCoordinationLeaseTTL: opts.UploadCoordinationLeaseTTL,
		passwordCost:               passwordCost,
		logger:                     logger,
	}
}
