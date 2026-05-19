// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// NodesHandler handles node directory endpoints.
type NodesHandler struct {
	pool  *pgxpool.Pool
	cache *nodeCache
}

// nodeCache provides in-memory caching for node list.
type nodeCache struct {
	mu         sync.RWMutex
	data       *NodeListResponse
	cachedAt   time.Time
	ttl        time.Duration
}

// NodeListResponse is the response structure for GET /v1/nodes.
type NodeListResponse struct {
	Nodes []NodeInfo `json:"nodes"`
	Total int        `json:"total"`
}

// NodeInfo represents a single node in the directory.
type NodeInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Tier        string   `json:"tier"`
	CountryCode string   `json:"country_code"`
	Region      string   `json:"region,omitempty"`
	City        string   `json:"city,omitempty"`
	ASN         string   `json:"asn,omitempty"`
	ISP         string   `json:"isp,omitempty"`
	Status      string   `json:"status"`
	// Lat/Lng come from enrolled_nodes.metadata (the agent geo annotator
	// populates them at enrollment time). Pointer so JSON omits the field
	// entirely when metadata lacks coords — the map UI uses presence as a
	// signal to draw the path origin.
	Lat *float64 `json:"lat,omitempty"`
	Lng *float64 `json:"lng,omitempty"`
}

// NewNodesHandler creates a new NodesHandler.
func NewNodesHandler(pool *pgxpool.Pool) *NodesHandler {
	return &NodesHandler{
		pool: pool,
		cache: &nodeCache{
			ttl: 5 * time.Minute,
		},
	}
}

// List returns a list of active nodes.
// GET /v1/nodes?country=CN&tier=tier1_cn
func (h *NodesHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	country := r.URL.Query().Get("country")
	tier := r.URL.Query().Get("tier")

	// Check cache first (only for queries without filters)
	if country == "" && tier == "" {
		if cached := h.cache.get(); cached != nil {
			response.JSON(w, r, http.StatusOK, cached)
			return
		}
	}

	// Query database
	nodes, err := h.queryNodes(ctx, country, tier)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query nodes", err))
		return
	}

	resp := &NodeListResponse{
		Nodes: nodes,
		Total: len(nodes),
	}

	// Cache the result if no filters were applied
	if country == "" && tier == "" {
		h.cache.set(resp)
	}

	response.JSON(w, r, http.StatusOK, resp)
}

// queryNodes queries the database for enrolled nodes matching the given filters.
func (h *NodesHandler) queryNodes(ctx context.Context, country, _ string) ([]NodeInfo, error) {
	// lat/lng guarded by a regex: a single malformed metadata row (e.g.
	// "lat":"unknown" from an old agent or a manual DB edit) used to throw
	// `invalid input syntax for type double precision` and 500 the entire
	// /v1/nodes endpoint — bad enough that the public map went dark.
	// NULLIF + regex returns NULL for non-numeric, which scans cleanly into
	// the *float64 destination and is omitted from JSON via omitempty.
	query := `
		SELECT node_id, hostname, ip_address, status,
		       metadata->>'country_code' AS country_code,
		       metadata->>'region'       AS region,
		       metadata->>'city'         AS city,
		       metadata->>'asn'          AS asn,
		       metadata->>'isp'          AS isp,
		       CASE WHEN metadata->>'lat' ~ '^-?[0-9]+(\.[0-9]+)?$'
		            THEN (metadata->>'lat')::float8 END AS lat,
		       CASE WHEN metadata->>'lng' ~ '^-?[0-9]+(\.[0-9]+)?$'
		            THEN (metadata->>'lng')::float8 END AS lng
		FROM enrolled_nodes
		WHERE status = 'active'
	`
	args := []any{}
	argIdx := 1

	if country != "" {
		query += " AND metadata->>'country_code' = $" + strconv.Itoa(argIdx)
		args = append(args, country)
		argIdx++
	}

	_ = argIdx
	query += " ORDER BY node_id LIMIT 1000"

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []NodeInfo
	for rows.Next() {
		var n NodeInfo
		var hostname, ipAddr *string
		var countryCode, region, city, asn, isp *string
		var lat, lng *float64

		if err := rows.Scan(
			&n.ID,
			&hostname,
			&ipAddr,
			&n.Status,
			&countryCode,
			&region,
			&city,
			&asn,
			&isp,
			&lat,
			&lng,
		); err != nil {
			return nil, err
		}

		if hostname != nil {
			n.Name = *hostname
		} else {
			n.Name = n.ID
		}
		if countryCode != nil {
			n.CountryCode = *countryCode
		}
		if region != nil {
			n.Region = *region
		}
		if city != nil {
			n.City = *city
		}
		if asn != nil {
			n.ASN = *asn
		}
		if isp != nil {
			n.ISP = *isp
		}
		n.Lat = lat
		n.Lng = lng
		n.Tier = "community"

		nodes = append(nodes, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if nodes == nil {
		nodes = []NodeInfo{}
	}

	return nodes, nil
}

// get returns cached data if it exists and is not expired.
func (c *nodeCache) get() *NodeListResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil || time.Since(c.cachedAt) > c.ttl {
		return nil
	}

	return c.data
}

// set updates the cache with new data.
func (c *nodeCache) set(data *NodeListResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = data
	c.cachedAt = time.Now()
}
