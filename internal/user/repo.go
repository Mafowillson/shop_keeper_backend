package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{col: db.Collection("users")}
}

func (repo *Repo) FindByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	filter := bson.M{"email": email}

	var user User

	err := repo.col.FindOne(ctx, filter).Decode(&user)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return User{}, mongo.ErrNoDocuments
		}

		return User{}, fmt.Errorf("find by email failed: %v", err)
	}

	return user, nil
}

func (repo *Repo) FindByID(ctx context.Context, id string) (User, error) {
	objectID, err := bson.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return User{}, fmt.Errorf("invalid user id: %w", err)
	}

	filter := bson.M{"_id": objectID}

	var user User
	if err := repo.col.FindOne(ctx, filter).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return User{}, mongo.ErrNoDocuments
		}
		return User{}, fmt.Errorf("find user by id failed: %v", err)
	}

	return user, nil
}

func (repo *Repo) Create(ctx context.Context, user User) (User, error) {

	res, err := repo.col.InsertOne(ctx, user)
	if err != nil {
		return User{}, fmt.Errorf("Insert user failed: %w", err)
	}

	id, ok := res.InsertedID.(bson.ObjectID)

	if !ok {
		return User{}, fmt.Errorf("Insert user failed and id is not objectid: %w", err)
	}

	user.ID = id

	return user, nil
}

func (repo *Repo) UpdateRefreshToken(ctx context.Context, userID string, refreshTokenHash string) error {
	objectID, err := bson.ObjectIDFromHex(strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	update := bson.M{
		"$set": bson.M{
			"refresh_token_hash": refreshTokenHash,
			"updated_at":         time.Now().UTC(),
		},
	}

	if _, err := repo.col.UpdateOne(ctx, bson.M{"_id": objectID}, update); err != nil {
		return fmt.Errorf("update refresh token failed: %w", err)
	}

	return nil
}

func (repo *Repo) ClearRefreshToken(ctx context.Context, userID string) error {
	objectID, err := bson.ObjectIDFromHex(strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	update := bson.M{
		"$unset": bson.M{"refresh_token_hash": ""},
		"$set":   bson.M{"updated_at": time.Now().UTC()},
	}

	if _, err := repo.col.UpdateOne(ctx, bson.M{"_id": objectID}, update); err != nil {
		return fmt.Errorf("clear refresh token failed: %w", err)
	}

	return nil
}
