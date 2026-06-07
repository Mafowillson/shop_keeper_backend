package sync

import "time"

// SyncRecord is a single offline operation submitted for reconciliation.
type SyncRecord struct {
	ID            string                 `json:"id"`
	DeviceID      string                 `json:"device_id"`
	EntityType    string                 `json:"entity_type"`    // sale | customer | debtRecord | product
	OperationType string                 `json:"operation_type"` // create | update | delete | payment
	Payload       map[string]interface{} `json:"payload"`
	Timestamp     time.Time              `json:"timestamp"`
}

// PushInput is the request body for POST /sync/push.
type PushInput struct {
	Records []SyncRecord `json:"records"`
}

// PushResult is the response body for POST /sync/push.
type PushResult struct {
	// Synced contains the entry IDs that were successfully committed.
	Synced []string `json:"synced"`
	// Conflicts contains the entry IDs that failed a business-rule check.
	// The client should show these to the user for manual review.
	Conflicts []string `json:"conflicts"`
	// IDMap maps each offline temp ID (offline_xxx) to the real server-assigned ID.
	// The client uses this to patch its local Hive cache.
	IDMap map[string]string `json:"id_map"`
}

// ProcessedEntry is stored in MongoDB to enforce idempotency across retries.
type ProcessedEntry struct {
	ID        string    `bson:"_id"`
	TempID    string    `bson:"temp_id,omitempty"`
	RealID    string    `bson:"real_id,omitempty"`
	CreatedAt time.Time `bson:"created_at"`
}
