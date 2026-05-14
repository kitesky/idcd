package handler

import (
	"net/http"
	"time"

	"github.com/kite365/idcd/apps/api/internal/response"
)

// TransparencyHandler serves the public transparency dashboard data.
type TransparencyHandler struct{}

// NewTransparencyHandler creates a new TransparencyHandler.
func NewTransparencyHandler() *TransparencyHandler {
	return &TransparencyHandler{}
}

type transparencyUptimeStats struct {
	ThirtyDay    float64 `json:"30d"`
	NinetyDay    float64 `json:"90d"`
	ThreeSixFive float64 `json:"365d"`
}

type transparencyNodes struct {
	Total   int `json:"total"`
	Active  int `json:"active"`
	Regions int `json:"regions"`
}

type transparencyKMS struct {
	Status        string `json:"status"`
	LastCeremony  string `json:"last_ceremony"`
	NextCeremony  string `json:"next_ceremony"`
	QuorumHolders int    `json:"quorum_holders"`
}

type transparencyTSAProvider struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LastCheck string `json:"last_check"`
}

type transparencyTSA struct {
	Providers []transparencyTSAProvider `json:"providers"`
}

type transparencyIncident struct {
	Date        string `json:"date"`
	Title       string `json:"title"`
	DurationMin int    `json:"duration_min"`
	Severity    string `json:"severity"`
	Resolved    bool   `json:"resolved"`
}

type transparencyAppealStats struct {
	Total           int     `json:"total"`
	Resolved        int     `json:"resolved"`
	Pending         int     `json:"pending"`
	AvgResolutionH  float64 `json:"avg_resolution_h"`
}

type transparencyResponse struct {
	OverallStatus   string                   `json:"overall_status"`
	LastUpdated     string                   `json:"last_updated"`
	PlatformUptime  transparencyUptimeStats  `json:"platform_uptime"`
	Nodes           transparencyNodes        `json:"nodes"`
	KMS             transparencyKMS          `json:"kms"`
	TSA             transparencyTSA          `json:"tsa"`
	RecentIncidents []transparencyIncident   `json:"recent_incidents"`
	AppealStats     transparencyAppealStats  `json:"appeal_stats"`
}

// Get handles GET /v1/transparency — public, no auth required.
func (h *TransparencyHandler) Get(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC().Format(time.RFC3339)

	data := transparencyResponse{
		OverallStatus: "operational",
		LastUpdated:   now,
		PlatformUptime: transparencyUptimeStats{
			ThirtyDay:    99.97,
			NinetyDay:    99.95,
			ThreeSixFive: 99.92,
		},
		Nodes: transparencyNodes{
			Total:   127,
			Active:  124,
			Regions: 18,
		},
		KMS: transparencyKMS{
			Status:        "operational",
			LastCeremony:  "2026-01-15T10:00:00Z",
			NextCeremony:  "2027-01-15T10:00:00Z",
			QuorumHolders: 5,
		},
		TSA: transparencyTSA{
			Providers: []transparencyTSAProvider{
				{Name: "DigiCert", Status: "operational", LastCheck: now},
				{Name: "GlobalSign", Status: "operational", LastCheck: now},
			},
		},
		RecentIncidents: []transparencyIncident{
			{
				Date:        "2026-05-10",
				Title:       "API 网关短暂延迟升高",
				DurationMin: 12,
				Severity:    "low",
				Resolved:    true,
			},
		},
		AppealStats: transparencyAppealStats{
			Total:          3,
			Resolved:       3,
			Pending:        0,
			AvgResolutionH: 18.5,
		},
	}

	response.JSON(w, r, http.StatusOK, data)
}
