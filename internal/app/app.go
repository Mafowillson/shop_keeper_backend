package app

import (
	"context"
	"fmt"
	"shop_keeper_backend/internal/config"
	"shop_keeper_backend/internal/db"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type App struct {
	config config.Config

	MongoClient *mongo.Client

	DB *mongo.Database
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

	return &App{
		config:      cfg,
		MongoClient: mongoCli.Client,
		DB:          mongoCli.DB,
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
