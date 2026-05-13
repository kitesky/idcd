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
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Tier        string `json:"tier"`
	CountryCode string `json:"country_code"`
	Region      string `json:"region,omitempty"`
	City        string `json:"city,omitempty"`
	ASN         string `json:"asn,omitempty"`
	ISP         string `json:"isp,omitempty"`
	Status      string `json:"status"`
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

// queryNodes queries the database for nodes matching the given filters.
func (h *NodesHandler) queryNodes(ctx context.Context, country, tier string) ([]NodeInfo, error) {
	query := `
		SELECT id, country, region, city, asn, asn_org, status, type, tier
		FROM node
		WHERE status = $1
	`
	args := []interface{}{"active"}
	argIdx := 2

	// Add country filter
	if country != "" {
		query += " AND country = $" + strconv.Itoa(argIdx)
		args = append(args, country)
		argIdx++
	}

	// Add tier filter (map tier1_cn/tier1_overseas/community to tier integer)
	if tier != "" {
		switch tier {
		case "tier1_cn", "tier1_overseas":
			query += " AND tier = $" + strconv.Itoa(argIdx)
			args = append(args, 1)
			argIdx++
		case "community":
			query += " AND tier = $" + strconv.Itoa(argIdx)
			args = append(args, 3)
			argIdx++
		}
	}

	query += " ORDER BY country, city LIMIT 1000"

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []NodeInfo
	for rows.Next() {
		var n NodeInfo
		var asnInt *int
		var tierInt *int
		var nodeType string

		err := rows.Scan(
			&n.ID,
			&n.CountryCode,
			&n.Region,
			&n.City,
			&asnInt,
			&n.ISP,
			&n.Status,
			&nodeType,
			&tierInt,
		)
		if err != nil {
			return nil, err
		}

		// Convert ASN integer to string
		if asnInt != nil {
			n.ASN = strconv.Itoa(*asnInt)
		}

		// Map tier integer to tier string
		if tierInt != nil {
			switch *tierInt {
			case 1:
				// Determine if CN or overseas based on country
				if n.CountryCode == "CN" {
					n.Tier = "tier1_cn"
				} else {
					n.Tier = "tier1_overseas"
				}
			case 2:
				n.Tier = "tier2"
			case 3:
				n.Tier = "community"
			}
		}

		// Use node ID as name if no explicit name field
		n.Name = n.ID

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
