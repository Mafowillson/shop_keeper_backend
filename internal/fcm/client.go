package fcm

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Client wraps the Firebase Messaging client.
// All notification sending in ShopKeeper goes through this struct.
type Client struct {
	messaging *messaging.Client
}

// NewClient initialises the Firebase Admin SDK using your service account JSON.
//
// How this works:
//  1. You download a service account key JSON from Firebase Console →
//     Project Settings → Service Accounts → Generate new private key.
//  2. You store that file's path in your .env as FIREBASE_CREDENTIALS_FILE.
//  3. This function reads that file, authenticates with Google, and returns
//     a messaging client ready to push notifications to any device.
func NewClient(ctx context.Context, credentialsFile string) (*Client, error) {
	// Tell the SDK where your service account key lives.
	opt := option.WithCredentialsFile(credentialsFile)

	// firebase.NewApp creates the root Firebase application instance.
	// nil config is fine — we only need Cloud Messaging, not Firestore/Auth.
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("fcm: failed to init firebase app: %w", err)
	}

	// app.Messaging() returns a client scoped to Firebase Cloud Messaging.
	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("fcm: failed to get messaging client: %w", err)
	}

	return &Client{messaging: msgClient}, nil
}

// SendToToken sends a push notification to a single device.
//
// Parameters:
//   - token : the FCM registration token stored on the owner's user record.
//     Flutter's FirebaseMessaging.instance.getToken() returns this.
//   - title : the bold heading shown in the device notification tray.
//   - body  : the message text below the title.
//   - data  : silent key-value pairs Flutter receives even when the app is
//     closed (onBackgroundMessage). Use this to pass IDs so Flutter
//     can navigate to the right screen when the user taps.
func (c *Client) SendToToken(
	ctx context.Context,
	token, title, body string,
	data map[string]string,
) error {
	message := &messaging.Message{
		// Token identifies which specific device to push to.
		Token: token,

		// Notification is the visible part the OS displays in the status bar.
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},

		// Data is the silent payload. Flutter reads this in onBackgroundMessage.
		Data: data,

		// Android config: HIGH priority wakes the device immediately.
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound: "default",
				// ChannelID must match the channel registered in your Flutter app.
				// We will use "shopkeeper_alerts" — make sure your Flutter app
				// creates this channel in MainActivity.kt using
				// flutter_local_notifications or firebase_messaging setup.
				ChannelID: "shopkeeper_alerts",
			},
		},
	}

	// Send dispatches the message to Firebase's servers.
	// Firebase then routes it to the device. We only care if there's an error.
	_, err := c.messaging.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("fcm: send to token failed: %w", err)
	}
	return nil
}
