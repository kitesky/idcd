package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// ManualReadyChannel is the Redis pub/sub channel that carries
// "user has installed their TXT record" notifications from the HTTP
// server (which owns the inbound API call) to the worker (which owns the
// in-process manual.Coordinator).
const ManualReadyChannel = "cert:manual_ready"

// ManualReadyMessage is the JSON payload published on ManualReadyChannel.
type ManualReadyMessage struct {
	OrderID int64  `json:"order_id"`
	FQDN    string `json:"fqdn"`
	Value   string `json:"value"`
}

// PublishManualReady is called by the HTTP handler in the server process
// when the user confirms a TXT record. It publishes a JSON-encoded
// ManualReadyMessage on Redis pub/sub; the worker process subscribes via
// RunManualReadySubscriber and forwards to the in-memory coordinator.
//
// Note this does NOT touch s.manualCoordinators directly because the
// server and the worker are different processes; the in-memory map is
// owned by whichever process is currently driving the order.
func (s *Service) PublishManualReady(ctx context.Context, orderID int64, fqdn, value string) error {
	if s.cfg.Redis == nil {
		return fmt.Errorf("publish manual ready: redis client not configured")
	}
	if orderID <= 0 {
		return fmt.Errorf("publish manual ready: invalid order id")
	}
	if fqdn == "" || value == "" {
		return fmt.Errorf("publish manual ready: fqdn and value required")
	}
	payload, err := json.Marshal(ManualReadyMessage{
		OrderID: orderID,
		FQDN:    fqdn,
		Value:   value,
	})
	if err != nil {
		return fmt.Errorf("publish manual ready: marshal: %w", err)
	}
	if err := s.cfg.Redis.Publish(ctx, ManualReadyChannel, payload).Err(); err != nil {
		return fmt.Errorf("publish manual ready: redis publish: %w", err)
	}
	return nil
}

// RunManualReadySubscriber subscribes to ManualReadyChannel and forwards
// every payload to MarkManualChallengeReady on the local Service — which
// signals the in-memory manual.Coordinator owning the order. The function
// blocks until ctx is cancelled; it returns ctx.Err() on shutdown and
// never propagates per-message parse failures (they are logged and the
// loop continues — a single garbled payload must not stall the bridge).
func (s *Service) RunManualReadySubscriber(ctx context.Context) error {
	if s.cfg.Redis == nil {
		return fmt.Errorf("manual ready subscriber: redis client not configured")
	}

	sub := s.cfg.Redis.Subscribe(ctx, ManualReadyChannel)
	defer func() { _ = sub.Close() }()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			s.handleManualReadyMessage(m)
		}
	}
}

// handleManualReadyMessage parses one pub/sub message and injects it into
// the local Coordinator. Errors are logged and swallowed so a malformed
// payload from another process can never kill the subscriber loop.
func (s *Service) handleManualReadyMessage(m *redis.Message) {
	if m == nil {
		return
	}
	var msg ManualReadyMessage
	if err := json.Unmarshal([]byte(m.Payload), &msg); err != nil {
		s.cfg.Logger.Warn("manual ready: bad payload",
			"channel", m.Channel, "err", err)
		return
	}
	if err := s.MarkManualChallengeReady(msg.OrderID, msg.FQDN, msg.Value); err != nil {
		// ErrManualCoordinator is expected when this worker is not the
		// one driving the order — log at debug, not warn.
		if errors.Is(err, ErrManualCoordinator) {
			s.cfg.Logger.Debug("manual ready: no local coordinator",
				"order_id", msg.OrderID)
			return
		}
		s.cfg.Logger.Warn("manual ready: inject failed",
			"order_id", msg.OrderID, "err", err)
	}
}
