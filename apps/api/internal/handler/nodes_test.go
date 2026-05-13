package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodesHandler_Cache(t *testing.T) {
	cache := &nodeCache{
		ttl: 0, // Expired immediately for testing
	}

	// Test cache miss
	result := cache.get()
	assert.Nil(t, result)

	// Test cache set and hit
	testData := &NodeListResponse{
		Nodes: []NodeInfo{
			{ID: "nd_test_01", Status: "active"},
		},
		Total: 1,
	}
	cache.set(testData)

	// With ttl=0, cache should be expired
	result = cache.get()
	assert.Nil(t, result)

	// Test cache with valid TTL
	cacheWithTTL := &nodeCache{
		ttl: 5 * time.Minute,
	}
	cacheWithTTL.set(testData)
	result = cacheWithTTL.get()
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "nd_test_01", result.Nodes[0].ID)
}

func TestNodeInfo_JSON(t *testing.T) {
	node := NodeInfo{
		ID:          "nd_cn_bj_01",
		Name:        "Beijing Node 1",
		Tier:        "tier1_cn",
		CountryCode: "CN",
		Region:      "Beijing",
		City:        "Beijing",
		ASN:         "4134",
		ISP:         "China Telecom",
		Status:      "active",
	}

	data, err := json.Marshal(node)
	require.NoError(t, err)

	var decoded NodeInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, node.ID, decoded.ID)
	assert.Equal(t, node.Tier, decoded.Tier)
	assert.Equal(t, node.CountryCode, decoded.CountryCode)
}

func TestNewNodesHandler(t *testing.T) {
	handler := NewNodesHandler(nil)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.cache)
	assert.Equal(t, 5*time.Minute, handler.cache.ttl)
}
