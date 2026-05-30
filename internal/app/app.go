package app

import (
	"context"
	"fmt"
	"shop_keeper_backend/internal/config"
	"shop_keeper_backend/internal/db"
	"shop_keeper_backend/internal/fcm"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type App struct {
	Config config.Config

	MongoClient *mongo.Client

	DB *mongo.Database

	FCMClient *fcm.Client
}

func New(ctx context.Context) (*App, error) {

	// load env
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// do the db connection second
	mongoCli, err := db.Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// init FCM client
	fcmClient, err := fcm.NewClient(ctx, cfg.FirebaseCredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to init firebase: %w", err)
	}

	return &App{
		Config:      cfg,
		MongoClient: mongoCli.Client,
		DB:          mongoCli.DB,
		FCMClient:   fcmClient,
	}, nil
}

func (app *App) Close(ctx context.Context) error {
	if app.MongoClient == nil {
		return nil
	}
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := app.MongoClient.Disconnect(closeCtx); err != nil {
		return fmt.Errorf("mongo disconnect failed: %w", err)
	}

	return nil
}
