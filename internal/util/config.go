package util

import "github.com/mxcd/go-config/config"

func InitConfig(version string) error {
	err := config.LoadConfig([]config.Value{
		config.String("DEPLOYMENT_IMAGE_TAG").NotEmpty().Default(version),

		config.String("LOG_LEVEL").NotEmpty().Default("info"),
		config.StringArray("ACCESS_LOGS").Default([]string{}), // api, ui

		config.Int("PORT").Default(8080),
		config.String("API_BASE_URL").Default("/api/v1"),
		config.String("HEALTH_ENDPOINT").Default("/health"),

		config.Bool("DEV").Default(false),
		config.Bool("STATIC_HOSTING").Default(true),
		config.String("UI_PROXY_URL").NotEmpty().Default("http://localhost:9000"),

		config.String("KUBECONFIG_PATH").Default(""),
		config.String("TARGET_NAMESPACE").Default(""),

		config.String("PSQL_BACKUP_IMAGE").Default("ghcr.io/go-cryo/cryo-psql:" + version),
		config.String("S3_BACKUP_IMAGE").Default("ghcr.io/go-cryo/cryo-s3:" + version),
		config.String("PVC_BACKUP_IMAGE").Default("ghcr.io/go-cryo/cryo-pvc:" + version),
	})
	return err
}
