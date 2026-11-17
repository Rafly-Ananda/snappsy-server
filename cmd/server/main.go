package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/db"
	ginHttp "github.com/rafly-ananda/snappsy-uploader-api/internal/http"
	adminHandler "github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/admin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/boomerangs"
	customerHandler "github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/customer"
	demoHandler "github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/demo"
	driveHandler "github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/drive"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/photos"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/middleware"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/migrations"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories/impls"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/websocket"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	objectStore, err := storage.NewSeaweedFS(
		cfg.StorageCfg.Endpoint,
		cfg.StorageCfg.PublicEndpoint,
		cfg.StorageCfg.AccessKey,
		cfg.StorageCfg.SecretKey,
		cfg.StorageCfg.Bucket,
		cfg.StorageCfg.UseSSL,
	)
	if err != nil {
		log.Fatalf("failed to init SeaweedFS: %v", err)
	}

	mongoStore, err := db.NewMongo(cfg.MongoCfg.Hosts, cfg.MongoCfg.DbUsername, cfg.MongoCfg.DbPassword, cfg.MongoCfg.DbName, cfg.MongoCfg.DbOpts)
	if err != nil {
		log.Fatalf("failed to init MongoDB: %v", err)
	}

	hub := websocket.NewHub()
	go hub.Run()

	mediaRepo := impls.NewMongoMediaRepository(mongoStore.Db.Collection(cfg.MongoCfg.ImageCollection))
	eventRepo := impls.NewMongoEventRepository(mongoStore.Db.Collection(cfg.MongoCfg.EventCollection))
	displayWebsocketHandler := websocket.NewDisplayWebSocketHandler(hub, eventRepo, cfg.GeneralCfg.AllowedOrigins)
	sessionRepo := impls.NewMongoAdminSessionRepository(mongoStore.Db.Collection("admin_sessions"))
	assetRepo := impls.NewMongoAssetRepository(mongoStore.Db.Collection(cfg.MongoCfg.AssetCollection))
	templateRepo := impls.NewMongoTemplateRepository(
		mongoStore.Db.Collection(cfg.MongoCfg.TemplateCollection),
		mongoStore.Db.Collection(cfg.MongoCfg.TemplateVersionCollection),
	)
	intentRepo := impls.NewMongoUploadIntentRepository(mongoStore.Db.Collection(cfg.MongoCfg.UploadIntentCollection))
	jobRepo := impls.NewMongoMediaJobRepository(mongoStore.Db.Collection(cfg.MongoCfg.MediaJobCollection))
	settingsRepo := impls.NewMongoPlatformSettingsRepository(mongoStore.Db.Collection(cfg.MongoCfg.PlatformSettingsCollection))
	driveRepo := impls.NewMongoDriveRepository(mongoStore.Db, cfg.MongoCfg.DriveCollection)
	customerRepo := impls.NewMongoCustomerRepository(mongoStore.Db.Collection(cfg.MongoCfg.CustomerCollection))
	customerSessionRepo := impls.NewMongoCustomerSessionRepository(mongoStore.Db.Collection(cfg.MongoCfg.CustomerSessionCollection))
	planRepo := impls.NewMongoEventPlanRepository(mongoStore.Db.Collection(cfg.MongoCfg.EventPlanCollection))

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer migrateCancel()
	if err := migrations.RunMongo(migrateCtx, mongoStore.Db, cfg.MongoCfg); err != nil {
		log.Fatalf("failed to run MongoDB migrations: %v", err)
	}

	indexCtx, indexCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer indexCancel()
	if err := mediaRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize media indexes: %v", err)
	}
	if err := eventRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize event indexes: %v", err)
	}
	if err := sessionRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize admin session indexes: %v", err)
	}
	if err := assetRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize asset indexes: %v", err)
	}
	if err := templateRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize template indexes: %v", err)
	}
	if err := intentRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize upload intent indexes: %v", err)
	}
	if err := jobRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize media job indexes: %v", err)
	}
	if err := driveRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize drive indexes: %v", err)
	}
	if err := settingsRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize settings indexes: %v", err)
	}
	if err := customerRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize customer indexes: %v", err)
	}
	if err := customerSessionRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize customer session indexes: %v", err)
	}
	if err := planRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to initialize event plan indexes: %v", err)
	}

	assetSvc := services.NewAssetService(assetRepo, objectStore, cfg.StorageCfg.Bucket, cfg.StorageCfg.PresignedExpiry)
	templateSvc := services.NewTemplateService(templateRepo, assetRepo)
	eventSvc := services.NewEventService(eventRepo, templateSvc.ValidateAssignment)
	adminSvc := services.NewAdminService(cfg.AdminCfg, sessionRepo)
	customerSvc := services.NewCustomerService(cfg.CustomerCfg, customerRepo, customerSessionRepo)
	planSvc := services.NewPlanService(planRepo)
	if err := planSvc.Seed(indexCtx); err != nil {
		log.Fatalf("failed to seed event plans: %v", err)
	}
	imageSvc := services.NewImageService(mediaRepo, eventRepo, objectStore, cfg.StorageCfg.Bucket, cfg.StorageCfg.PresignedExpiry)
	driveSvc := services.NewDriveService(cfg.DriveCfg, driveRepo, eventRepo, mediaRepo, objectStore, cfg.StorageCfg.Bucket)
	photoSvc := services.NewPhotoService(mediaRepo, intentRepo, eventRepo, templateRepo, assetRepo, objectStore, cfg.StorageCfg.Bucket, cfg.StorageCfg.PresignedExpiry, hub, driveSvc)
	boomerangSvc := services.NewBoomerangService(mediaRepo, intentRepo, jobRepo, eventRepo, objectStore, cfg.StorageCfg.Bucket, cfg.StorageCfg.PresignedExpiry, cfg.BoomerangCfg.SegmentSeconds, cfg.BoomerangCfg.MaxSourceBytes, cfg.BoomerangCfg.QueueLimit)

	workerMode := strings.ToLower(strings.TrimSpace(os.Getenv("WORKER_MODE")))
	if workerMode != "" && workerMode != "false" && workerMode != "0" {
		workerCtx, workerCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer workerCancel()
		if workerMode == "true" || workerMode == "all" || workerMode == "boomerang" {
			log.Println("starting boomerang worker")
			boomerangWorker := services.NewBoomerangWorker(jobRepo, mediaRepo, objectStore, cfg.BoomerangCfg, cfg.StorageCfg.Bucket, driveSvc)
			boomerangWorker.SetEventRepository(eventRepo)
			boomerangWorker.SetTemplateRepository(templateRepo)
			boomerangWorker.SetAssetRepository(assetRepo)
			boomerangWorker.SetHub(hub)
			go boomerangWorker.Run(workerCtx)
		}
		if workerMode == "all" || workerMode == "drive" {
			log.Println("starting drive export worker")
			go driveSvc.Run(workerCtx)
		}
		<-workerCtx.Done()
		return
	}

	photoHandler := photos.NewPhotoHandler(photoSvc)
	boomerangHandler := boomerangs.NewBoomerangHandler(boomerangSvc)
	driveHandler := driveHandler.NewDriveHandler(driveSvc)
	publicEventHandler := events.NewPublicEventHandler(eventSvc, imageSvc)
	adminH := adminHandler.NewAdminHandler(adminSvc, eventSvc)
	adminPlanH := adminHandler.NewPlanHandler(planSvc, eventSvc)
	customerH := customerHandler.NewHandler(customerSvc, planSvc, eventSvc, photoSvc, driveSvc, assetSvc, templateSvc)
	settingsSvc := services.NewSettingsService(settingsRepo, cfg.DemoCfg)
	demoH := demoHandler.NewHandler(services.NewDemoConvertService(cfg.BoomerangCfg.FFmpegPath), settingsSvc, assetSvc, templateSvc)
	adminSettingsH := adminHandler.NewSettingsHandler(settingsSvc)
	assetHandler := adminHandler.NewAssetHandler(assetSvc)
	templateHandler := adminHandler.NewTemplateHandler(templateSvc, eventSvc)

	r := ginHttp.NewRouter(ginHttp.Handlers{
		Photos:           photoHandler,
		Boomerangs:       boomerangHandler,
		PublicEvents:     publicEventHandler,
		Admin:            adminH,
		AdminAssets:      assetHandler,
		AdminTemplates:   templateHandler,
		Drive:            driveHandler,
		DisplayWebsocket: displayWebsocketHandler,
		AdminAuth:        middleware.AdminRequired(adminSvc),
		AdminCSRF:        middleware.AdminCSRF(adminSvc),
		Customer:         customerH,
		Demo:             demoH,
		AdminSettings:    adminSettingsH,
		AdminPlans:       adminPlanH,
		CustomerAuth:     middleware.CustomerRequired(customerSvc),
		CustomerCSRF:     middleware.CustomerCSRF(customerSvc),
	}, cfg.GeneralCfg.AllowedOrigins)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "live", "version": Version})
	})
	r.GET("/readyz", func(c *gin.Context) {
		readyCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := mongoStore.Ping(readyCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "dependency": "mongodb"})
			return
		}
		if err := objectStore.Ping(readyCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "dependency": "seaweedfs"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	srv := &http.Server{
		Addr:              ":" + cfg.GeneralCfg.GinPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Starting server version %s (built %s) on :%s", Version, BuildTime, cfg.GeneralCfg.GinPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	mongoCtx, mongoCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer mongoCancel()
	if err := mongoStore.Disconnect(mongoCtx); err != nil {
		log.Printf("MongoDB disconnect error: %v", err)
	}
	log.Println("bye")
}
