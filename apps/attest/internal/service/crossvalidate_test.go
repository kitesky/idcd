package service

import (
	"context"
	"testing"
)

func TestCrossValidate_AllOK(t *testing.T) {
	obs := []observation{
		{NodeID: "a", OK: true},
		{NodeID: "b", OK: true},
		{NodeID: "c", OK: true},
	}
	nodes, pct := crossValidate(context.Background(), obs)
	if len(nodes) != 3 {
		t.Fatalf("nodes = %v, want 3 entries", nodes)
	}
	if pct != 100 {
		t.Fatalf("consistency = %v, want 100", pct)
	}
}

func TestCrossValidate_PartialFailures(t *testing.T) {
	obs := []observation{
		{NodeID: "a", OK: true},
		{NodeID: "b", OK: false},
	}
	_, pct := crossValidate(context.Background(), obs)
	if pct != 50 {
		t.Fatalf("consistency = %v, want 50", pct)
	}
}

func TestCrossValidate_Empty(t *testing.T) {
	nodes, pct := crossValidate(context.Background(), nil)
	if nodes != nil {
		t.Fatalf("nodes = %v, want nil", nodes)
	}
	if pct != 0 {
		t.Fatalf("consistency = %v, want 0", pct)
	}
}
