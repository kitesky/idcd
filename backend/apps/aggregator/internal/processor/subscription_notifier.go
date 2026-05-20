package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

type SubscriptionNotifier struct {
	db      pgxQuerier
	enqueue func(ctx context.Context, payload any) error
	logger  *slog.Logger
}

func NewSubscriptionNotifier(db pgxQuerier, enqueue func(ctx context.Context, payload any) error, logger *slog.Logger) *SubscriptionNotifier {
	return &SubscriptionNotifier{db: db, enqueue: enqueue, logger: logger}
}

type subscriptionEventPayload struct {
	ChannelType string `json:"channel_type"`
	Endpoint    string `json:"endpoint"`
	EventType   string `json:"event_type"`
	MonitorID   string `json:"monitor_id"`
}

func (s *SubscriptionNotifier) NotifySubscribers(ctx context.Context, monitorID string, eventType string) error {
	statusPageID, err := s.findStatusPageForMonitor(ctx, monitorID)
	if err != nil {
		return nil
	}
	if statusPageID == "" {
		return nil
	}

	rows, err := s.db.Query(ctx, `
		SELECT channel_type, endpoint
		FROM status_page_subscriptions
		WHERE status_page_id = $1
		  AND verified = TRUE
		  AND $2 = ANY(events)
	`, statusPageID, eventType)
	if err != nil {
		return fmt.Errorf("subscription_notifier: query subscriptions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var channelType, endpoint string
		if err := rows.Scan(&channelType, &endpoint); err != nil {
			s.logger.Error("subscription_notifier: scan row", "err", err)
			continue
		}

		p := subscriptionEventPayload{
			ChannelType: channelType,
			Endpoint:    endpoint,
			EventType:   eventType,
			MonitorID:   monitorID,
		}
		data, err := json.Marshal(p)
		if err != nil {
			s.logger.Error("subscription_notifier: marshal payload", "err", err)
			continue
		}

		if s.enqueue != nil {
			if err := s.enqueue(ctx, data); err != nil {
				s.logger.Error("subscription_notifier: enqueue", "channel_type", channelType, "err", err)
			}
		}
	}

	return rows.Err()
}

func (s *SubscriptionNotifier) findStatusPageForMonitor(ctx context.Context, monitorID string) (string, error) {
	var statusPageID string
	err := s.db.QueryRow(ctx, `
		SELECT spc.status_page_id
		FROM status_page_components spc
		WHERE spc.monitor_id = $1
		LIMIT 1
	`, monitorID).Scan(&statusPageID)
	if err != nil {
		return "", err
	}
	return statusPageID, nil
}
