package postmortem

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func ptr(t time.Time) *time.Time { return &t }

func TestGenerateDraft_LowSeverity(t *testing.T) {
	start := time.Now().Add(-3 * time.Minute)
	resolved := start.Add(3 * time.Minute)
	draft := GenerateDraft(DraftInput{
		MonitorName: "API Gateway",
		MonitorType: "http",
		StartedAt:   start,
		ResolvedAt:  ptr(resolved),
		Duration:    3 * time.Minute,
	})
	assert.Equal(t, "low", draft.Severity)
	assert.Contains(t, draft.Title, "[low]")
}

func TestGenerateDraft_MediumSeverity(t *testing.T) {
	start := time.Now().Add(-20 * time.Minute)
	resolved := start.Add(20 * time.Minute)
	draft := GenerateDraft(DraftInput{
		MonitorName: "Database",
		MonitorType: "http",
		StartedAt:   start,
		ResolvedAt:  ptr(resolved),
		Duration:    20 * time.Minute,
	})
	assert.Equal(t, "medium", draft.Severity)
}

func TestGenerateDraft_CriticalSeverity(t *testing.T) {
	start := time.Now().Add(-3 * time.Hour)
	resolved := start.Add(3 * time.Hour)
	draft := GenerateDraft(DraftInput{
		MonitorName: "Payment Service",
		MonitorType: "http",
		StartedAt:   start,
		ResolvedAt:  ptr(resolved),
		Duration:    3 * time.Hour,
	})
	assert.Equal(t, "critical", draft.Severity)
}

func TestGenerateDraft_HighSeverity(t *testing.T) {
	start := time.Now().Add(-90 * time.Minute)
	resolved := start.Add(90 * time.Minute)
	draft := GenerateDraft(DraftInput{
		MonitorName: "Auth Service",
		MonitorType: "http",
		StartedAt:   start,
		ResolvedAt:  ptr(resolved),
		Duration:    90 * time.Minute,
	})
	assert.Equal(t, "high", draft.Severity)
}

func TestGenerateDraft_HTTPMonitorActionItems(t *testing.T) {
	draft := GenerateDraft(DraftInput{
		MonitorName: "Web App",
		MonitorType: "http",
		StartedAt:   time.Now(),
		Duration:    10 * time.Minute,
	})
	found := false
	for _, item := range draft.ActionItems {
		if strings.Contains(item.Item, "健康检查") {
			found = true
			break
		}
	}
	assert.True(t, found, "http monitor should have 健康检查 action item")
}

func TestGenerateDraft_DNSMonitorActionItems(t *testing.T) {
	draft := GenerateDraft(DraftInput{
		MonitorName: "DNS Check",
		MonitorType: "dns",
		StartedAt:   time.Now(),
		Duration:    10 * time.Minute,
	})
	found := false
	for _, item := range draft.ActionItems {
		if strings.Contains(item.Item, "DNS") {
			found = true
			break
		}
	}
	assert.True(t, found, "dns monitor should have DNS action item")
}

func TestGenerateDraft_TitleContainsMonitorName(t *testing.T) {
	draft := GenerateDraft(DraftInput{
		MonitorName: "MyService",
		MonitorType: "http",
		StartedAt:   time.Now(),
		Duration:    10 * time.Minute,
	})
	assert.Contains(t, draft.Title, "MyService")
}

func TestGenerateDraft_UnresolvedNoResolution(t *testing.T) {
	draft := GenerateDraft(DraftInput{
		MonitorName: "API",
		MonitorType: "http",
		StartedAt:   time.Now(),
		Duration:    10 * time.Minute,
		ResolvedAt:  nil,
	})
	assert.Equal(t, "", draft.Resolution)
	assert.Equal(t, "待恢复", draft.Timeline[1].Time)
}

func TestGenerateDraft_ResolvedHasResolution(t *testing.T) {
	start := time.Now().Add(-30 * time.Minute)
	resolved := start.Add(30 * time.Minute)
	draft := GenerateDraft(DraftInput{
		MonitorName: "API",
		MonitorType: "http",
		StartedAt:   start,
		ResolvedAt:  ptr(resolved),
		Duration:    30 * time.Minute,
	})
	assert.NotEmpty(t, draft.Resolution)
	assert.Contains(t, draft.Resolution, "恢复")
}
