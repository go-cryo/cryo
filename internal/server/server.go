package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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
		config.AllowOrigins = []string{"http://localhost:8080"}
		config.AllowCredentials = true
		server.Engine.Use(cors.New(config))
	}

	return server, nil
}

func (s *Server) RegisterRoutes() error {
	s.registerHealthRoute()
	s.registerVersionRoute()
	if s.HostProvider != nil {
		s.registerHostRoutes()
	}
	if s.RepositoryProvider != nil {
		s.registerRepositoryRoutes()
	}
	if s.BackupJobProvider != nil {
		s.registerBackupJobRoutes()
	}
	if s.SettingsProvider != nil {
		s.registerSettingsRoutes()
	}

	event.RegisterWebsocketManager(&event.WebsocketOptions{
		ApiBaseUrl: s.Options.ApiBaseUrl,
		Engine:     s.Engine,
	})

	// must be the last route to be registered
	web.RegisterUI(&web.WebHostingOptions{
		DevMode:       s.Options.DevMode,
		StaticHosting: s.Options.StaticHosting,
		UIProxyUrl:    s.Options.UiProxyUrl,
		Engine:        s.Engine,
	})

	return nil
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
