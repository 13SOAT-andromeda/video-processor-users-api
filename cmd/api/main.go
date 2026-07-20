package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/dd-trace-go/v2/profiler"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.uber.org/zap"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database"
	usermodel "github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/repository"
	httpAdapter "github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/handlers"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/services"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer func() {
		tracer.Stop()
		profiler.Stop()
		_ = logger.Sync()
	}()

	sugar := logger.Sugar()

	cfg, err := config.Init()
	if err != nil {
		sugar.Fatalf("failed to load config: %v", err)
	}

	if err = profiler.Start(
		profiler.WithEnv(cfg.Env),
		profiler.WithService(cfg.Service),
		profiler.WithVersion(cfg.Version),
		profiler.WithProfileTypes(profiler.CPUProfile, profiler.HeapProfile),
	); err != nil {
		sugar.Warnf("datadog profiler unavailable: %v", err)
	}

	if err = tracer.Start(
		tracer.WithEnv(cfg.Env),
		tracer.WithService(cfg.Service),
		tracer.WithServiceVersion(cfg.Version),
	); err != nil {
		sugar.Warnf("datadog tracer unavailable: %v", err)
	}

	ctx := context.Background()

	// JWT signing key é buscado uma vez aqui, antes do servidor começar a
	// aceitar requisições — nunca dentro de um handler por request — mesmo
	// padrão de video-processor-authorizer/cmd/authorizer/main.go e
	// video-processor-authentication-api/cmd/api/main.go.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		sugar.Fatalf("failed to load AWS config: %v", err)
	}
	secretsClient := secretsmanager.NewFromConfig(awsCfg)
	secretValue, err := secretsClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &cfg.JWT.SigningKeySecretName,
	})
	if err != nil {
		sugar.Fatalf("failed to load jwt signing key: %v", err)
	}
	jwtSecret := *secretValue.SecretString

	db, err := database.Init(ctx, *cfg.Database)
	if err != nil {
		sugar.Fatalf("failed to connect database: %v", err)
	}

	if err = db.AutoMigrate(&usermodel.Model{}); err != nil {
		sugar.Fatalf("failed to migrate: %v", err)
	}

	if err = database.SeedAdmin(ctx, db.GetDB(), database.AdminSeedConfig{
		Email:    cfg.AdminUser.Email,
		Document: cfg.AdminUser.Document,
	}); err != nil {
		sugar.Warnf("seed admin: %v", err)
	}

	userRepo := repository.NewUserRepository(db.GetDB())
	userService := services.NewUserService(userRepo)
	userHandler := handlers.NewUserHandler(userService)

	router := httpAdapter.NewRouter(*cfg, logger, *userHandler, jwtSecret)

	sugar.Infof("starting server on port %s", cfg.Http.Port)
	if err = router.Server(":" + cfg.Http.Port); err != nil {
		sugar.Fatalf("server failed: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM)
	go func() {
		<-sigChan
		tracer.Stop()
	}()
}
