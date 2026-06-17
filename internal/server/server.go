package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-cryo/cryo/internal/auth"
	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/event"
	"github.com/go-cryo/cryo/internal/executor"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/repositoryhost"
	"github.com/go-cryo/cryo/internal/restic"
	"github.com/go-cryo/cryo/internal/scheduler"
	"github.com/go-cryo/cryo/internal/settings"
	"github.com/go-cryo/cryo/internal/web"
	"github.com/rs/zerolog/log"
)

type ServerOptions struct {
	ServiceVersion     string
	DevMode            bool
	Port               int
	ApiBaseUrl         string
	StaticHosting      bool
	UiProxyUrl         string
	AccessLogs         []string
	HealthEndpoint     string
	HostProvider       repositoryhost.Provider
	RepositoryProvider repository.Provider
	ResticService      *restic.Service
	BackupJobProvider  backupjob.Provider
	Scheduler          *scheduler.Scheduler
	RunStore           *executor.RunStore
	SettingsProvider   settings.Provider
	BasicAuthHandler   *auth.BasicAuthHandler
	OidcHandler        *auth.OIDCHandler
}

type Server struct {
	Options            *ServerOptions
	Engine             *gin.Engine
	HttpServer         *http.Server
	HostProvider       repositoryhost.Provider
	RepositoryProvider repository.Provider
	ResticService      *restic.Service
	BackupJobProvider  backupjob.Provider
	Scheduler          *scheduler.Scheduler
	RunStore           *executor.RunStore
	SettingsProvider   settings.Provider
	BasicAuthHandler   *auth.BasicAuthHandler
	OidcHandler        *auth.OIDCHandler
}

func NewServer(options *ServerOptions) (*Server, error) {
	if options == nil {
		return nil, fmt.Errorf("server options cannot be nil")
	}

	server := &Server{
		Options:            options,
		HostProvider:       options.HostProvider,
		RepositoryProvider: options.RepositoryProvider,
		ResticService:      options.ResticService,
		BackupJobProvider:  options.BackupJobProvider,
		Scheduler:          options.Scheduler,
		RunStore:           options.RunStore,
		SettingsProvider:   options.SettingsProvider,
		BasicAuthHandler:   options.BasicAuthHandler,
		OidcHandler:        options.OidcHandler,
	}

	if !server.Options.DevMode {
		log.Info().Msg("Running Gin in production mode")
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	server.Engine = engine
	server.Engine.Use(gin.Recovery(), server.zeroLogger())
	server.HttpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", options.Port),
		Handler: engine,
	}

	if server.Options.DevMode {
		log.Info().Msg("Running Gin in development mode")
		log.Warn().Msg("CORS is enabled for all origins")
		config := cors.DefaultConfig()
		config.AllowHeaders = []string{"Authorization", "Content-Type", "X-Requested-With", "X-PINGOTHER", "X-File-Name", "Cache-Control"}
		config.AllowOrigins = []string{"http://localhost:8080", "http://localhost:9000"}
		config.AllowCredentials = true
		server.Engine.Use(cors.New(config))
	}

	return server, nil
}

func (s *Server) RegisterRoutes() error {
	// Public routes — always accessible without auth
	s.registerHealthRoute()
	s.registerVersionRoute()
	s.registerAuthInfoRoute()

	// Apply combined auth middleware — protects /api/* routes only
	s.Engine.Use(s.combinedAuth())

	// Protected API routes
	apiGroup := s.Engine.Group(s.Options.ApiBaseUrl)

	// Session probe used by the SPA router guard. It sits behind combinedAuth,
	// so it returns 200 for any valid session (BasicAuth or OIDC) and 401
	// otherwise — unlike /auth/me, which only the BasicAuth handler serves.
	s.registerAuthSessionRoute(apiGroup)

	if s.HostProvider != nil {
		s.registerHostRoutes(apiGroup)
	}
	if s.RepositoryProvider != nil {
		s.registerRepositoryRoutes(apiGroup)
	}
	if s.BackupJobProvider != nil {
		s.registerBackupJobRoutes(apiGroup)
	}
	if s.SettingsProvider != nil {
		s.registerSettingsRoutes(apiGroup)
	}

	// WebSocket
	event.RegisterWebsocketManager(&event.WebsocketOptions{
		ApiBaseUrl: s.Options.ApiBaseUrl,
		Engine:     s.Engine,
	})

	// UI hosting — must be last (catch-all, no auth middleware)
	web.RegisterUI(s.Engine, s.Options.DevMode, s.Options.StaticHosting, s.Options.UiProxyUrl)

	return nil
}

// combinedAuth returns a middleware that protects /api/* routes by checking
// BasicAuth and OIDC sessions in sequence. Non-API paths (UI) are always
// allowed through so the SPA can load. Public API endpoints are whitelisted.
func (s *Server) combinedAuth() gin.HandlerFunc {
	oidcEnabled := s.OidcHandler != nil
	basicAuthEnabled := s.BasicAuthHandler != nil

	if !oidcEnabled && !basicAuthEnabled {
		return func(c *gin.Context) { c.Next() }
	}

	publicPaths := []string{
		s.Options.HealthEndpoint,
		s.Options.ApiBaseUrl + s.Options.HealthEndpoint,
		s.Options.ApiBaseUrl + "/version",
		s.Options.ApiBaseUrl + "/auth/info",
		s.Options.ApiBaseUrl + "/auth/login",
		s.Options.ApiBaseUrl + "/auth/logout",
		s.Options.ApiBaseUrl + "/auth/me",
	}

	apiPrefix := s.Options.ApiBaseUrl

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip non-API paths — UI routes are not protected server-side. Use the
		// configured API base so a custom API_BASE_URL is still guarded.
		if path != apiPrefix && !strings.HasPrefix(path, apiPrefix+"/") {
			c.Next()
			return
		}

		// Public API endpoints that don't require authentication
		for _, p := range publicPaths {
			if path == p {
				c.Next()
				return
			}
		}

		// 1. Try BasicAuth session
		if basicAuthEnabled && s.BasicAuthHandler.CheckSession(c.Request) {
			c.Next()
			return
		}

		// 2. Try OIDC session
		if oidcEnabled && s.OidcHandler.CheckSession(c.Request) {
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		c.Abort()
	}
}

func (s *Server) Run() error {
	if err := s.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) {
	s.HttpServer.Shutdown(ctx)
}

func (s *Server) accessLogEnabled(category string) bool {
	for _, c := range s.Options.AccessLogs {
		if strings.TrimSpace(c) == category {
			return true
		}
	}
	return false
}

func (s *Server) isAPIRequest(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws")
}

func (s *Server) zeroLogger() gin.HandlerFunc {
	logAPI := s.accessLogEnabled("api")
	logUI := s.accessLogEnabled("ui")

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		isAPI := s.isAPIRequest(path)
		if isAPI && !logAPI {
			return
		}
		if !isAPI && !logUI {
			return
		}

		status := c.Writer.Status()
		latency := time.Since(start)

		log.Trace().
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Str("client_ip", c.ClientIP()).
			Str("latency", latency.String()).
			Msg("http_request")
	}
}
