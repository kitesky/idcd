package processor

import (
	"context"
	"io"
	"log/slog"
	"testing"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func newTestNotifier(t *testing.T) (pgxmock.PgxPoolIface, []any, *SubscriptionNotifier) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	var enqueued []any
	enqueue := func(_ context.Context, payload any) error {
		enqueued = append(enqueued, payload)
		return nil
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewSubscriptionNotifier(mock, enqueue, logger)
	return mock, enqueued, n
}

func TestSubscriptionNotifier_NoStatusPage_NoNotification(t *testing.T) {
	mock, _, notifier := newTestNotifier(t)
	ctx := context.Background()
	monitorID := "m_nopage001"

	mock.ExpectQuery("SELECT spc.status_page_id").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"status_page_id"}))

	err := notifier.NotifySubscribers(ctx, monitorID, "incident")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSubscriptionNotifier_WithSubscribers_EnqueuesNotification(t *testing.T) {
	mock, _, notifier := newTestNotifier(t)
	var enqueuedPayloads []any
	notifier.enqueue = func(_ context.Context, payload any) error {
		enqueuedPayloads = append(enqueuedPayloads, payload)
		return nil
	}

	ctx := context.Background()
	monitorID := "m_page001"
	statusPageID := "sp_page001"

	mock.ExpectQuery("SELECT spc.status_page_id").
		WithArgs(monitorID).
		WillReturnRows(pgxmock.NewRows([]string{"status_page_id"}).AddRow(statusPageID))

	mock.ExpectQuery("SELECT channel_type, endpoint").
		WithArgs(statusPageID, "incident").
		WillReturnRows(pgxmock.NewRows([]string{"channel_type", "endpoint"}).
			AddRow("email", "user@example.com").
			AddRow("webhook", "https://hooks.example.com/notify"))

	err := notifier.NotifySubscribers(ctx, monitorID, "incident")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(enqueuedPayloads) != 2 {
		t.Errorf("expected 2 enqueued notifications, got %d", len(enqueuedPayloads))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
