package chat

import "time"

type Message struct {
	ID        string    `bson:"_id"`
	OwnerID   string    `bson:"owner_id"`
	Role      string    `bson:"role"` // "user" or "model"
	Text      string    `bson:"text"`
	CreatedAt time.Time `bson:"created_at"`
}

type SendRequest struct {
	Message string `json:"message" binding:"required"`
}

type MessageResponse struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type SendResponse struct {
	Reply   string          `json:"reply"`
	Message MessageResponse `json:"message"`
}

type HistoryResponse struct {
	Messages []MessageResponse `json:"messages"`
}
