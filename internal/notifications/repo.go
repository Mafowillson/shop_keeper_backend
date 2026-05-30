package notification

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Repo handles all MongoDB operations for the notification module.
// It talks directly to the "notifications" and "notification_preferences"
// collections. No business logic lives here — only database calls.
type Repo struct {
	notifications *mongo.Collection
	preferences   *mongo.Collection
	users         *mongo.Collection // needed to look up the owner's FCM token
}

// NewRepo constructs a Repo wired to the given database.
// Call this in main.go / app.go when wiring dependencies.
func NewRepo(db *mongo.Database) *Repo {
	return &Repo{
		notifications: db.Collection("notifications"),
		preferences:   db.Collection("notification_preferences"),
		users:         db.Collection("users"),
	}
}

// -----------------------------------------------------------------------
// Notification CRUD
// -----------------------------------------------------------------------

// Save inserts a new notification document.
// Called by the service every time an event fires (low stock, large sale, etc.).
func (r *Repo) Save(ctx context.Context, n *Notification) error {
	_, err := r.notifications.InsertOne(ctx, n)
	if err != nil {
		return fmt.Errorf("notification repo: save: %w", err)
	}
	return nil
}

// ListByOwner returns paginated notifications for one owner.
//
// Parameters:
//   - ownerID  : only fetch notifications belonging to this owner
//   - unreadOnly : when true, add a filter so only unread docs are returned
//   - limit / skip : standard pagination (e.g. limit=20, skip=0 for page 1)
//
// Results are sorted newest-first so the inbox always shows the latest event
// at the top.
func (r *Repo) ListByOwner(
	ctx context.Context,
	ownerID bson.ObjectID,
	unreadOnly bool,
	limit, skip int64,
) ([]Notification, error) {
	// Build the filter. We always filter by owner_id.
	filter := bson.M{"owner_id": ownerID}

	// Optionally add the unread filter (FR-27: ?unread=true query param).
	if unreadOnly {
		filter["read"] = false
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}). // newest first
		SetLimit(limit).
		SetSkip(skip)

	cursor, err := r.notifications.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("notification repo: list: %w", err)
	}
	defer cursor.Close(ctx)

	var results []Notification
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("notification repo: decode list: %w", err)
	}
	return results, nil
}

// CountUnread returns how many unread notifications the owner has.
// Used to populate a badge count in Flutter's bottom nav bar.
func (r *Repo) CountUnread(ctx context.Context, ownerID bson.ObjectID) (int64, error) {
	count, err := r.notifications.CountDocuments(ctx, bson.M{
		"owner_id": ownerID,
		"read":     false,
	})
	if err != nil {
		return 0, fmt.Errorf("notification repo: count unread: %w", err)
	}
	return count, nil
}

// MarkRead sets read=true on a single notification.
// FR-28: PATCH /notifications/:id/read
//
// We also check that the notification belongs to the requesting owner
// (ownerID) so one owner cannot mark another's notifications as read.
func (r *Repo) MarkRead(ctx context.Context, id, ownerID bson.ObjectID) error {
	filter := bson.M{
		"_id":      id,
		"owner_id": ownerID, // ownership guard
	}
	update := bson.M{
		"$set": bson.M{"read": true},
	}

	result, err := r.notifications.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("notification repo: mark read: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("notification repo: not found or access denied")
	}
	return nil
}

// MarkAllRead sets read=true on every unread notification for an owner.
// Useful for a "mark all as read" button in Flutter.
func (r *Repo) MarkAllRead(ctx context.Context, ownerID bson.ObjectID) error {
	_, err := r.notifications.UpdateMany(ctx,
		bson.M{"owner_id": ownerID, "read": false},
		bson.M{"$set": bson.M{"read": true}},
	)
	if err != nil {
		return fmt.Errorf("notification repo: mark all read: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------
// Preferences CRUD
// -----------------------------------------------------------------------

// GetPreferences fetches the owner's notification preferences.
// If no document exists yet (new owner), it returns default preferences.
func (r *Repo) GetPreferences(ctx context.Context, ownerID bson.ObjectID) (*Preferences, error) {
	var prefs Preferences
	err := r.preferences.FindOne(ctx, bson.M{"owner_id": ownerID}).Decode(&prefs)

	if err == mongo.ErrNoDocuments {
		// First time: return defaults without saving — the owner must call
		// PUT /notifications/preferences to explicitly save them.
		defaults := DefaultPreferences(ownerID)
		return &defaults, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notification repo: get preferences: %w", err)
	}
	return &prefs, nil
}

// UpsertPreferences saves or updates the owner's preferences.
// Uses MongoDB upsert so it works whether or not a document already exists.
func (r *Repo) UpsertPreferences(ctx context.Context, prefs *Preferences) error {
	prefs.ID = bson.NewObjectID() // harmless if overwritten by upsert

	opts := options.Replace().SetUpsert(true)
	_, err := r.preferences.ReplaceOne(
		ctx,
		bson.M{"owner_id": prefs.OwnerID},
		prefs,
		opts,
	)
	if err != nil {
		return fmt.Errorf("notification repo: upsert preferences: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------
// FCM token helpers
// -----------------------------------------------------------------------

// GetOwnerFCMToken fetches the FCM device token stored on the owner's user doc.
// The token was saved when Flutter called POST /owner/fcm-token on app start.
//
// Why store it on the users collection?
// Because users already exists and the token belongs to a specific user (owner).
// Adding one field there is simpler than creating a separate tokens collection.
func (r *Repo) GetOwnerFCMToken(ctx context.Context, ownerID bson.ObjectID) (string, error) {
	var result struct {
		FCMToken string `bson:"fcm_token"`
	}

	err := r.users.FindOne(
		ctx,
		bson.M{"_id": ownerID},
		options.FindOne().SetProjection(bson.M{"fcm_token": 1}),
	).Decode(&result)

	if err != nil {
		return "", fmt.Errorf("notification repo: get fcm token: %w", err)
	}
	return result.FCMToken, nil
}

// SaveFCMToken writes the FCM token to the owner's user document.
// Called when Flutter POSTs to /api/v1/owner/fcm-token.
func (r *Repo) SaveFCMToken(ctx context.Context, ownerID bson.ObjectID, token string) error {
	_, err := r.users.UpdateOne(
		ctx,
		bson.M{"_id": ownerID},
		bson.M{
			"$set": bson.M{
				"fcm_token":         token,
				"fcm_token_updated": time.Now(),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("notification repo: save fcm token: %w", err)
	}
	return nil
}
