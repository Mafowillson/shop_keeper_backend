package db

import (
	"context"
	"fmt"
	"shop_keeper_backend/internal/config"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Mongo struct {
	Client *mongo.Client

	DB *mongo.Database
}

func Connect(ctx context.Context, cfg config.Config) (*Mongo, error) {

	conntectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	clientOpts := options.Client().
		ApplyURI(cfg.MongoURI).
		SetBSONOptions(&options.BSONOptions{
			// Existing User documents stored shop_id as bson.ObjectID.
			// After migrating the field to string, we need the driver to
			// decode ObjectID values as their 24-char hex representation
			// so those rows don't break FindByEmail / FindByID.
			ObjectIDAsHexString: true,
		})

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongo connection failed: %w", err)
	}

	// Ping verifies the server is actually reachable at startup
	// This catches misconfigured URIs or a down server immediately
	if err := client.Ping(conntectCtx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping failed: %w", err)
	}

	database := client.Database(cfg.MongoDBName)

	return &Mongo{
		Client: client,
		DB:     database,
	}, nil
}
