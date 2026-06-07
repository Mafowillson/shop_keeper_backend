package sync

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const collectionName = "synced_entries"

type Repo struct {
	db *mongo.Database
}

func NewRepo(db *mongo.Database) *Repo {
	r := &Repo{db: db}
	// Sparse index on temp_id — only documents that carry a temp_id are indexed,
	// keeping the index small while making GetProcessedByTempID an O(log n) lookup.
	ctx := context.Background()
	_, _ = r.col().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "temp_id", Value: 1}},
		Options: options.Index().SetSparse(true),
	})
	return r
}

func (r *Repo) col() *mongo.Collection {
	return r.db.Collection(collectionName)
}

// GetProcessed returns (true, realID, nil) when the entry has already been
// processed.  Returns (false, "", nil) when the entry is new.  Returns a
// non-nil error only on infrastructure failures (e.g. MongoDB unavailable);
// callers should abort the entire batch in that case.
func (r *Repo) GetProcessed(ctx context.Context, entryID string) (bool, string, error) {
	var entry ProcessedEntry
	if err := r.col().FindOne(ctx, bson.M{"_id": entryID}).Decode(&entry); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, entry.RealID, nil
}

// GetProcessedByTempID looks up a previously committed entry by its offline
// temp entity ID (e.g. "offline_0abc_1f3a").  Used to resolve cross-batch
// entity references.  Returns an empty realID if no match is found; returns
// a non-nil error only on infrastructure failures.
func (r *Repo) GetProcessedByTempID(ctx context.Context, tempID string) (string, error) {
	var entry ProcessedEntry
	if err := r.col().FindOne(ctx, bson.M{"temp_id": tempID}).Decode(&entry); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", nil
		}
		return "", err
	}
	return entry.RealID, nil
}

// MarkProcessed records that an entry was committed.  Silently ignores
// duplicate inserts so that the caller never needs to guard against them.
func (r *Repo) MarkProcessed(ctx context.Context, entryID, tempID, realID string) error {
	entry := ProcessedEntry{
		ID:        entryID,
		TempID:    tempID,
		RealID:    realID,
		CreatedAt: time.Now().UTC(),
	}
	_, err := r.col().InsertOne(ctx, entry)
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	return err
}
