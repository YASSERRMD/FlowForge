package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/YASSERRMD/FlowForge/internal/apierror"
	"github.com/YASSERRMD/FlowForge/internal/config"
)

// Identity is the authenticated principal attached to a request.
type Identity struct {
	Subject     string   `json:"subject"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

type identityKey struct{}

// IdentityFrom returns the authenticated identity, or nil.
func IdentityFrom(ctx context.Context) *Identity {
	if id, ok := ctx.Value(identityKey{}).(*Identity); ok {
		return id
	}
	return nil
}

// Authenticator resolves a request into an Identity. Implementations must
// fail closed: any doubt means an error, never an anonymous principal.
type Authenticator interface {
	Authenticate(r *http.Request) (*Identity, error)
}

// ErrUnauthenticated is returned when no valid credentials are present.
var ErrUnauthenticated = errors.New("no valid credentials")

// DevAuthenticator is the explicit development-only identity boundary. It
// trusts a header exactly once: when APP_ENV=development AND
// AUTH_DEV_MODE=true. Any other configuration must never construct it.
type DevAuthenticator struct{}

// DevUserHeader carries the dev-mode identity. It is ignored unless the
// DevAuthenticator is explicitly wired in development.
const DevUserHeader = "X-Dev-User"

// Authenticate implements Authenticator.
func (DevAuthenticator) Authenticate(r *http.Request) (*Identity, error) {
	subject := r.Header.Get(DevUserHeader)
	if subject == "" {
		return nil, ErrUnauthenticated
	}
	return &Identity{
		Subject:     "dev:" + subject,
		DisplayName: subject,
		Roles:       []string{"developer"},
	}, nil
}

// NewAuthenticator returns the authenticator for the given config. In
// development with AUTH_DEV_MODE=true it returns the dev boundary; in every
// other case it returns nil, meaning "no auth mechanism configured" — the
// RequireAuth middleware then fails closed with 401 on every request.
func NewAuthenticator(cfg config.Config) Authenticator {
	if cfg.Env == config.EnvDevelopment && cfg.DevAuthEnabled {
		return DevAuthenticator{}
	}
	return nil
}

// RequireAuth guards a route group. A nil authenticator fails closed:
// requests are rejected with 401 rather than silently allowed through.
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			apierror.Write(w, r, s.log, apierror.New(
				apierror.CodeUnauthorized,
				"authentication is not configured for this environment",
			))
			return
		}
		identity, err := s.auth.Authenticate(r)
		if err != nil {
			apierror.Write(w, r, s.log, apierror.New(
				apierror.CodeUnauthorized, "authentication required"))
			return
		}
		ctx := context.WithValue(r.Context(), identityKey{}, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	identity := IdentityFrom(r.Context())
	if identity == nil {
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeUnauthorized, "authentication required"))
		return
	}
	apierror.JSON(w, http.StatusOK, identity)
}
