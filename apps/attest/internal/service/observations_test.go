package service

import (
	"context"
	"testing"
)

func TestFetchObservations_NilOrderRejected(t *testing.T) {
	if _, err := fetchObservations(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil order")
	}
}

func TestFetchObservations_HappyShape(t *testing.T) {
	obs, err := fetchObservations(context.Background(), &Order{ID: "vo_x"})
	if err != nil {
		t.Fatalf("fetchObservations: %v", err)
	}
	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}
}
