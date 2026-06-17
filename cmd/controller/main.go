package main

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/go-cryo/cryo/internal/auth"
	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/executor"
	"github.com/go-cryo/cryo/internal/kubernetes"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/repositoryhost"
	"github.com/go-cryo/cryo/internal/restic"
	"github.com/go-cryo/cryo/internal/scheduler"
	"github.com/go-cryo/cryo/internal/server"
	"github.com/go-cryo/cryo/internal/settings"
	"github.com/go-cryo/cryo/internal/util"
	"github.com/mxcd/go-config/config"
	"k8s.io/client-go/dynamic"
)

var version = "development"

func main() {
	ctx := context.Background()

	if err := util.InitConfig(version); err != nil {
		log.Panic().Err(err).Msg("error initializing config")
	}
	config.Print()

	if err := util.InitLogger(); err != nil {
		log.Panic().Err(err).Msg("error initializing logger")
	}

	kubernetesClient, err := kubernetes.NewClient(&kubernetes.ClientOptions{
		Namespace:      config.Get().String("TARGET_NAMESPACE"),
		KubeconfigPath: config.Get().String("KUBECONFIG_PATH"),
	})
	if err != nil {
		log.Panic().Err(err).Msg("error initializing Kubernetes client")
	}

	namespace := config.Get().String("TARGET_NAMESPACE")
	hostProvider := repositoryhost.NewKubernetesProvider(kubernetesClient.ClientSet, namespace)
	repoProvider := repository.NewKubernetesProvider(kubernetesClient.ClientSet, namespace, hostProvider)
	resticService := restic.NewService()

	backupJobProvider := backupjob.NewKubernetesProvider(kubernetesClient.ClientSet, namespace)
	runStore := executor.NewRunStore(kubernetesClient.ClientSet)

	dynamicClient, err := dynamic.NewForConfig(kubernetesClient.RestConfig)
	if err != nil {
		log.Panic().Err(err).Msg("error creating dynamic Kubernetes client")
	}

	settingsProvider := settings.NewKubernetesProvider(kubernetesClient.ClientSet, namespace)

	pvcOrchestrator := executor.NewPVCOrchestrator(
		kubernetesClient.ClientSet,
		dynamicClient,
		repoProvider,
		config.Get().String("PVC_BACKUP_IMAGE"),
		settingsProvider,
	)

	exec := executor.NewExecutor(
		kubernetesClient.ClientSet,
		repoProvider,
		runStore,
		pvcOrchestrator,
		&executor.ExecutorOptions{
			PSQLBackupImage: config.Get().String("PSQL_BACKUP_IMAGE"),
			S3BackupImage:   config.Get().String("S3_BACKUP_IMAGE"),
			PVCBackupImage:  config.Get().String("PVC_BACKUP_IMAGE"),
		},
		settingsProvider,
	)

	sched := scheduler.NewScheduler(backupJobProvider, exec)

	ensureRepoInitialized := func(ns, name string) {
		ctx := context.Background()
		repo, err := repoProvider.Get(ctx, ns, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", ns).Str("name", name).Msg("failed to resolve repository")
			return
		}
		status, err := resticService.Check(ctx, repo)
		if err != nil {
			log.Error().Err(err).Str("repository", name).Msg("failed to check repository")
			return
		}
		if status.OK {
			log.Info().Str("namespace", ns).Str("name", name).Msg("repository verified")
			return
		}
		log.Info().Str("namespace", ns).Str("name", name).Msg("repository not initialized, running restic init")
		if err := resticService.Init(ctx, repo); err != nil {
			log.Error().Err(err).Str("namespace", ns).Str("name", name).Msg("failed to initialize repository")
			return
		}
		log.Info().Str("namespace", ns).Str("name", name).Msg("repository initialized successfully")
	}

	err = kubernetesClient.StartNotifier(&kubernetes.NotifierOptions{
		BackupJob: &kubernetes.WatchCallbacks{
			OnAdd: func(ns, name string) {
				sched.SyncJob(context.Background(), ns, name)
			},
			OnUpdate: func(ns, name string) {
				sched.SyncJob(context.Background(), ns, name)
			},
			OnDelete: func(ns, name string) {
				sched.RemoveJob(ns, name)
			},
		},
		Repository: &kubernetes.WatchCallbacks{
			OnAdd:    ensureRepoInitialized,
			OnUpdate: ensureRepoInitialized,
		},
	})
	if err != nil {
		log.Panic().Err(err).Msg("error starting kubernetes notifier")
	}

	if err := sched.SyncAll(ctx); err != nil {
		log.Error().Err(err).Msg("error syncing backup job schedules")
	}
	sched.Start()

	srv := initServer(&InitServerOptions{
		HostProvider:       hostProvider,
		RepositoryProvider: repoProvider,
		ResticService:      resticService,
		BackupJobProvider:  backupJobProvider,
		Scheduler:          sched,
		RunStore:           runStore,
		SettingsProvider:   settingsProvider,
		KubernetesClient:   kubernetesClient,
		Namespace:          namespace,
	})

	err = srv.Run()
	if err != nil {
		log.Panic().Err(err).Msg("error running server")
	}
}

type InitServerOptions struct {
	HostProvider       repositoryhost.Provider
	RepositoryProvider repository.Provider
	ResticService      *restic.Service
	BackupJobProvider  backupjob.Provider
	Scheduler          *scheduler.Scheduler
	RunStore           *executor.RunStore
	SettingsProvider   settings.Provider
	KubernetesClient   *kubernetes.Client
	Namespace          string
}

func initServer(options *InitServerOptions) *server.Server {
	devMode := config.Get().Bool("DEV")
	apiBaseUrl := config.Get().String("API_BASE_URL")

	srv, err := server.NewServer(&server.ServerOptions{
		ServiceVersion:     version,
		DevMode:            devMode,
		Port:               config.Get().Int("PORT"),
		AccessLogs:         config.Get().StringArray("ACCESS_LOGS"),
		ApiBaseUrl:         apiBaseUrl,
		HealthEndpoint:     config.Get().String("HEALTH_ENDPOINT"),
		StaticHosting:      config.Get().Bool("STATIC_HOSTING"),
		UiProxyUrl:         config.Get().String("UI_PROXY_URL"),
		HostProvider:       options.HostProvider,
		RepositoryProvider: options.RepositoryProvider,
		ResticService:      options.ResticService,
		BackupJobProvider:  options.BackupJobProvider,
		Scheduler:          options.Scheduler,
		RunStore:           options.RunStore,
		SettingsProvider:   options.SettingsProvider,
	})
	if err != nil {
		log.Panic().Err(err).Msg("error initializing server")
	}

	// Initialize auth handlers and register routes on the engine
	basicEnabled := config.Get().Bool("AUTH_BASIC_ENABLED")
	oidcEnabled := config.Get().Bool("AUTH_OIDC_ENABLED")

	if basicEnabled || oidcEnabled {
		sessionKeys, err := auth.EnsureSessionKeys(
			options.KubernetesClient.ClientSet,
			options.Namespace,
			config.Get().String("AUTH_SESSION_SECRET_NAME"),
		)
		if err != nil {
			log.Panic().Err(err).Msg("error ensuring session keys")
		}

		if oidcEnabled {
			scopes := strings.Split(config.Get().String("OIDC_SCOPES"), ",")
			oidcHandler, err := auth.NewOIDCHandler(srv.Engine, &auth.OIDCHandlerOptions{
				Issuer:       config.Get().String("OIDC_ISSUER"),
				ClientID:     config.Get().String("OIDC_CLIENT_ID"),
				ClientSecret: config.Get().String("OIDC_CLIENT_SECRET"),
				RedirectURI:  config.Get().String("OIDC_REDIRECT_URI"),
				Scopes:       scopes,
				Role:         config.Get().String("OIDC_ROLE"),
				RolePath:     config.Get().String("OIDC_ROLE_PATH"),
				SessionKeys:  sessionKeys,
				DevMode:      devMode,
			})
			if err != nil {
				log.Panic().Err(err).Msg("error setting up OIDC auth")
			}
			srv.OidcHandler = oidcHandler
		}

		if basicEnabled {
			basicAuthHandler, err := auth.NewBasicAuthHandler(srv.Engine, options.KubernetesClient.ClientSet, options.Namespace, &auth.BasicAuthHandlerOptions{
				AdminUsername:    config.Get().String("AUTH_ADMIN_USERNAME"),
				AdminSecretName: config.Get().String("AUTH_ADMIN_SECRET_NAME"),
				SessionKeys:     sessionKeys,
				DevMode:         devMode,
				ApiBaseUrl:      apiBaseUrl,
			})
			if err != nil {
				log.Panic().Err(err).Msg("error setting up basic auth")
			}
			srv.BasicAuthHandler = basicAuthHandler
		}
	}

	err = srv.RegisterRoutes()
	if err != nil {
		log.Panic().Err(err).Msg("error registering routes")
	}

	return srv
}
