package scheduler_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/kite365/idcd/apps/scheduler/internal/queue"
	"github.com/kite365/idcd/apps/scheduler/internal/scheduler"
)

func TestDBNodeSelector_ReturnsActiveNode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT node_id`).
		WillReturnRows(pgxmock.NewRows([]string{"node_id"}).AddRow("nd_abc123"))

	sel := scheduler.NewDBNodeSelectorFromQuerier(mock)
	nodeID, err := sel.SelectNode(context.Background(), &queue.ProbeTask{})
	if err != nil {
		t.Fatalf("SelectNode: %v", err)
	}
	if nodeID != "nd_abc123" {
		t.Errorf("want nd_abc123, got %q", nodeID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDBNodeSelector_NoActiveNodes(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT node_id`).
		WillReturnError(pgx.ErrNoRows)

	sel := scheduler.NewDBNodeSelectorFromQuerier(mock)
	_, err = sel.SelectNode(context.Background(), &queue.ProbeTask{})
	if err == nil {
		t.Error("expected error when no active nodes, got nil")
	}
}
