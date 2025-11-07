package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/db"
	ginHttp "github.com/rafly-ananda/snappsy-uploader-api/internal/http"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/images"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories/impls"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/websocket"
)

func main() {
	// Projector calls GET /sessions/:id/images?limit=200 repeatedly or (better) you push via WebSocket using Redis Streams as discussed earlier.
	cfg := config.Load()

	minioStore, err := storage.NewMinio(cfg.MinioCfg.MinIOEndpoint, cfg.MinioCfg.MinIOAccessKey, cfg.MinioCfg.MinIOSecretKey, cfg.MinioCfg.MinIOBucket, cfg.MinioCfg.MinioPresignedExpiry, false)
	if err != nil {
		log.Fatalf("failed to init minio: %v", err)
	}

	mongoStore, err := db.NewMongo(cfg.MongoCfg.Hosts, cfg.MongoCfg.DbUsername, cfg.MongoCfg.DbPassword, cfg.MongoCfg.DbName, cfg.MongoCfg.DbOpts)
	if err != nil {
		log.Fatalf("failed to init mongo: %v", err)
	}

	// Websocket Initialization
	hub := websocket.NewHub()
	go hub.Run()
	websocketHandler := websocket.NewWebSocketHandler(hub)

	// Repository Initialization
	imageRepo := impls.NewMongoImageRepository(mongoStore.Db.Collection(cfg.MongoCfg.ImageCollection))
	eventRepo := impls.NewMongoEventRepository(mongoStore.Db.Collection(cfg.MongoCfg.EventCollection))

	// Service Initialization
	imageSvc := services.NewImageService(imageRepo, minioStore, cfg.MinioCfg.MinIOBucket, cfg.MinioCfg.MinioPresignedExpiry, hub)
	eventSvc := services.NewEventService(eventRepo)

	// Handler Initialization
	imageHandler := images.NewImageHandler(imageSvc)
	eventHandler := events.NewEventHandler(eventSvc)

	r := ginHttp.NewRouter(ginHttp.Handlers{
		Images: imageHandler,
		Events: eventHandler,
		Websocket: websocketHandler,
	})

	srv := &http.Server{
		Addr:    ":5000",
		Handler: r,
	}

	// Start server
	go func() {
		log.Printf("starting server on :%s", cfg.GeneralCfg.GinPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("bye")
}
