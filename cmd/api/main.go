package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/dd-trace-go/v2/profiler"
	"go.uber.org/zap"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/authclient"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database"
	usermodel "github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/repository"
	httpAdapter "github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/handlers"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/services"
	"github.com/13SOAT-andromeda/video-processor-users-api/pkg/encryption"
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
	db, err := database.Init(ctx, *cfg.Database)
	if err != nil {
		sugar.Fatalf("failed to connect database: %v", err)
	}

	if err = db.AutoMigrate(&usermodel.Model{}); err != nil {
		sugar.Fatalf("failed to migrate: %v", err)
	}

	if err = database.SeedAdmin(ctx, db.GetDB(), database.AdminSeedConfig{
		Email:    cfg.AdminUser.Email,
		Password: cfg.AdminUser.Password,
		Document: cfg.AdminUser.Document,
	}); err != nil {
		sugar.Warnf("seed admin: %v", err)
	}

	// Adapters
	userRepo := repository.NewUserRepository(db.GetDB())
	authClient := authclient.NewAuthServiceClient(cfg.Auth.ServiceURL)
	hasher := encryption.NewBcryptHasher()

	// Services
	userService := services.NewUserService(userRepo, authClient, hasher)

	// Handlers
	userHandler := handlers.NewUserHandler(userService)

	// Router
	router := httpAdapter.NewRouter(*cfg, logger, *userHandler)

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
