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

func (r *Repo) FindByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u User
	if err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return User{}, mongo.ErrNoDocuments
		}
		return User{}, fmt.Errorf("find by email: %w", err)
	}
	return u, nil
}

func (r *Repo) FindByID(ctx context.Context, id string) (User, error) {
	oid, err := bson.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return User{}, fmt.Errorf("invalid user id: %w", err)
	}
	var u User
	if err := r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return User{}, mongo.ErrNoDocuments
		}
		return User{}, fmt.Errorf("find by id: %w", err)
	}
	return u, nil
}

func (r *Repo) Create(ctx context.Context, u User) (User, error) {
	res, err := r.col.InsertOne(ctx, u)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	id, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return User{}, fmt.Errorf("create user: inserted id is not ObjectID")
	}
	u.ID = id
	return u, nil
}

func (r *Repo) UpdateShopID(ctx context.Context, userID, shopID string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{"shop_id": shopID, "updated_at": time.Now().UTC()},
	})
	return err
}

func (r *Repo) UpdateRefreshToken(ctx context.Context, userID, hash string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{"refresh_token_hash": hash, "updated_at": time.Now().UTC()},
	})
	return err
}

func (r *Repo) ClearRefreshToken(ctx context.Context, userID string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$unset": bson.M{"refresh_token_hash": ""},
		"$set":   bson.M{"updated_at": time.Now().UTC()},
	})
	return err
}

// ── Email verification ────────────────────────────────────────────────────────

func (r *Repo) SaveVerificationCode(ctx context.Context, userID, code string, expiry time.Time) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"verification_code":        code,
			"verification_code_expiry": expiry,
			"verification_sent_at":     now,
			"updated_at":               now,
		},
	})
	return err
}

func (r *Repo) MarkEmailVerified(ctx context.Context, userID string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set":   bson.M{"email_verified": true, "updated_at": time.Now().UTC()},
		"$unset": bson.M{"verification_code": "", "verification_code_expiry": "", "verification_sent_at": ""},
	})
	return err
}

// ── Password reset ────────────────────────────────────────────────────────────

func (r *Repo) SavePasswordResetCode(ctx context.Context, userID, code string, expiry time.Time) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"password_reset_code":     code,
			"password_reset_expiry":   expiry,
			"password_reset_sent_at":  now,
			"updated_at":              now,
		},
	})
	return err
}

func (r *Repo) ResetPassword(ctx context.Context, userID, newPasswordHash string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"PasswordHash": newPasswordHash,
			"updated_at":   time.Now().UTC(),
		},
		"$unset": bson.M{
			"password_reset_code":    "",
			"password_reset_expiry":  "",
			"password_reset_sent_at": "",
			// Invalidate all active sessions so the old password can't be reused.
			"refresh_token_hash": "",
		},
	})
	return err
}
