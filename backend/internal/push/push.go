package push

import (
	"context"
	"encoding/json"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/rs/zerolog"
)

// Subscription holds a browser push subscription for one device.
type Subscription struct {
	ID       string
	UserID   string
	Endpoint string
	P256DH   string
	Auth     string
}

// Notification is the payload delivered to the browser.
type Notification struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Store is the data-access interface required by the push service.
type Store interface {
	Save(ctx context.Context, sub Subscription) error
	Delete(ctx context.Context, userID, endpoint string) error
	ListByUser(ctx context.Context, userID string) ([]Subscription, error)
	DeleteByEndpoint(ctx context.Context, endpoint string) error
}

// senderFunc is the signature of webpush.SendNotificationWithContext so it can
// be replaced in tests.
type senderFunc func(ctx context.Context, message []byte, s *webpush.Subscription, options *webpush.Options) (*http.Response, error)

// Service manages push subscriptions and delivers web push notifications.
type Service struct {
	store        Store
	vapidPublic  string
	vapidPrivate string
	vapidSubject string
	sender       senderFunc
	log          zerolog.Logger
}

// NewService creates a push Service. If vapidPublic or vapidPrivate are empty,
// the service is disabled and Send becomes a no-op.
func NewService(store Store, vapidPublic, vapidPrivate, vapidSubject string, log zerolog.Logger) *Service {
	return &Service{
		store:        store,
		vapidPublic:  vapidPublic,
		vapidPrivate: vapidPrivate,
		vapidSubject: vapidSubject,
		sender:       webpush.SendNotificationWithContext,
		log:          log.With().Str("component", "push").Logger(),
	}
}

// Enabled reports whether VAPID credentials are configured.
func (s *Service) Enabled() bool {
	return s.vapidPublic != "" && s.vapidPrivate != ""
}

// VAPIDPublicKey returns the public VAPID key for the browser to use when
// subscribing.
func (s *Service) VAPIDPublicKey() string { return s.vapidPublic }

// Subscribe saves a new push subscription for the user. If the endpoint
// already exists it is updated in place (UNIQUE constraint upsert).
func (s *Service) Subscribe(ctx context.Context, userID, endpoint, p256dh, auth string) error {
	return s.store.Save(ctx, Subscription{
		UserID:   userID,
		Endpoint: endpoint,
		P256DH:   p256dh,
		Auth:     auth,
	})
}

// Unsubscribe removes a specific subscription by endpoint for the user.
func (s *Service) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	return s.store.Delete(ctx, userID, endpoint)
}

// Send delivers n to every active subscription for userID. Delivery failures
// are logged and skipped. Subscriptions that have expired (HTTP 404/410) are
// removed automatically. Send is a no-op when the service is disabled.
// It is safe to call from a goroutine.
func (s *Service) Send(ctx context.Context, userID string, n Notification) {
	if !s.Enabled() {
		return
	}

	log := s.log.With().Str("user_id", userID).Logger()

	subs, err := s.store.ListByUser(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("list subscriptions failed")
		return
	}
	log.Debug().Int("subscriptions", len(subs)).Msg("sending push notification")
	if len(subs) == 0 {
		return
	}

	payload, err := json.Marshal(n)
	if err != nil {
		log.Error().Err(err).Msg("marshal notification failed")
		return
	}

	opts := &webpush.Options{
		VAPIDPublicKey:  s.vapidPublic,
		VAPIDPrivateKey: s.vapidPrivate,
		Subscriber:      s.vapidSubject,
		TTL:             60,
	}

	for _, sub := range subs {
		resp, err := s.sender(ctx, payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256DH,
				Auth:   sub.Auth,
			},
		}, opts)
		if err != nil {
			log.Error().Err(err).Str("endpoint", sub.Endpoint).Msg("send failed")
			continue
		}
		log.Debug().Int("status", resp.StatusCode).Str("endpoint", sub.Endpoint).Msg("sent")
		resp.Body.Close()

		// 404 / 410 mean the subscription is no longer valid — remove it.
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			if err := s.store.DeleteByEndpoint(ctx, sub.Endpoint); err != nil {
				log.Error().Err(err).Str("endpoint", sub.Endpoint).Msg("delete stale subscription failed")
			}
		}
	}
}
