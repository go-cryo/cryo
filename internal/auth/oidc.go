package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mxcd/oidc-fwd-auth/pkg/oidc"
	"github.com/rs/zerolog/log"
)

type OIDCHandlerOptions struct {
	Issuer          string
	ClientID        string
	ClientSecret    string
	RedirectURI     string
	Scopes          []string
	Role            string // required Keycloak role
	RolePath        string // JSON path to roles array in claims (e.g. "realm_access.roles")
	SessionKeys     *SessionKeys
	DevMode         bool
	BaseContextPath string // default "/auth/oidc"
}

// OIDCHandler wraps oidc-fwd-auth and provides session checking for the
// combined auth middleware. Routes are registered during creation.
type OIDCHandler struct {
	handler *oidc.Handler
	role    string
	rolePath string
}

// LoginURL returns the OIDC login URL for the frontend to link to.
func (h *OIDCHandler) LoginURL() string {
	return h.handler.Options.AuthBaseContextPath + "/login"
}

// NewOIDCHandler creates the OIDC handler and registers routes on the engine.
// Routes are registered at baseContextPath (default /auth/oidc):
//   - GET /auth/oidc/login — initiate OIDC flow
//   - GET /auth/oidc/callback — OAuth2 callback
func NewOIDCHandler(engine *gin.Engine, opts *OIDCHandlerOptions) (*OIDCHandler, error) {
	if opts.Issuer == "" || opts.ClientID == "" || opts.ClientSecret == "" || opts.RedirectURI == "" {
		return nil, fmt.Errorf("OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, and OIDC_REDIRECT_URI are required")
	}

	baseContextPath := opts.BaseContextPath
	if baseContextPath == "" {
		baseContextPath = "/auth/oidc"
	}

	handler, err := oidc.NewHandler(&oidc.Options{
		Provider: &oidc.ProviderOptions{
			Issuer:       opts.Issuer,
			ClientId:     opts.ClientID,
			ClientSecret: opts.ClientSecret,
			RedirectUri:  opts.RedirectURI,
			Scopes:       opts.Scopes,
		},
		Session: &oidc.SessionOptions{
			SecretSigningKey:    string(opts.SessionKeys.SigningKey),
			SecretEncryptionKey: string(opts.SessionKeys.EncryptionKey),
			Name:                "cryo_oidc_session",
			Domain:              "",
			MaxAge:              86400,
			Secure:              !opts.DevMode,
		},
		AuthBaseContextPath: baseContextPath,
	})
	if err != nil {
		return nil, fmt.Errorf("creating OIDC handler: %w", err)
	}

	// Register a middleware to handle OIDC state mismatch gracefully.
	// When the server restarts during an in-flight OIDC login, the in-memory
	// session cache is wiped and the callback will fail with "state mismatch".
	// This middleware intercepts that case and redirects to login instead of
	// letting the library return a JSON error.
	callbackPath := baseContextPath + "/callback"
	engine.Use(func(c *gin.Context) {
		if c.Request.URL.Path != callbackPath {
			c.Next()
			return
		}
		state := c.Query("state")
		savedState, _ := handler.SessionStore.GetStringValue(c.Request, "state")
		if state == "" || savedState == "" || state != savedState {
			log.Warn().
				Str("state", state).
				Str("savedState", savedState).
				Msg("OIDC state mismatch — clearing session and redirecting to login")
			// Clear the OIDC session cookie so the next login starts fresh
			http.SetCookie(c.Writer, &http.Cookie{
				Name:     "cryo_oidc_session",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
			})
			c.Redirect(http.StatusFound, baseContextPath+"/login")
			c.Abort()
			return
		}
		c.Next()
	})

	handler.RegisterRoutes(engine)

	log.Info().Str("issuer", opts.Issuer).Msg("OIDC auth enabled")

	return &OIDCHandler{
		handler:  handler,
		role:     opts.Role,
		rolePath: opts.RolePath,
	}, nil
}

// CheckSession returns true if the request has a valid OIDC session with the
// required role (if configured).
func (h *OIDCHandler) CheckSession(r *http.Request) bool {
	sessionData, err := h.handler.SessionStore.GetSessionData(r)
	if err != nil || sessionData == nil || !sessionData.Authenticated {
		return false
	}
	if h.role != "" && !hasRole(sessionData.Claims, h.rolePath, h.role) {
		log.Debug().
			Str("role", h.role).
			Str("rolePath", h.rolePath).
			Msg("OIDC session valid but user lacks required role")
		return false
	}
	return true
}

// hasRole navigates a nested claims map using a dot-separated path and checks
// if the specified role exists in the resulting array.
func hasRole(claims map[string]interface{}, path, role string) bool {
	if claims == nil {
		return false
	}

	parts := strings.Split(path, ".")
	var current interface{} = claims

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return false
		}
		current, ok = m[part]
		if !ok {
			return false
		}
	}

	roles, ok := current.([]interface{})
	if !ok {
		return false
	}

	for _, r := range roles {
		if s, ok := r.(string); ok && s == role {
			return true
		}
	}
	return false
}
