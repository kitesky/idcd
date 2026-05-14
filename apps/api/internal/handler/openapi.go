package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"go.yaml.in/yaml/v2"

	apidocs "github.com/kite365/idcd/apps/api/docs"
)

// OpenAPIHandler serves the OpenAPI spec as JSON.
type OpenAPIHandler struct {
	once        sync.Once
	cachedJSON  []byte
	cachedError error
}

// NewOpenAPIHandler creates a new OpenAPIHandler.
func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{}
}

// OpenAPI handles GET /v1/openapi.json
func (h *OpenAPIHandler) OpenAPI(w http.ResponseWriter, r *http.Request) {
	h.once.Do(func() {
		h.cachedJSON, h.cachedError = yamlToJSON(apidocs.Spec)
	})

	if h.cachedError != nil {
		http.Error(w, "failed to process OpenAPI spec", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.cachedJSON)
}

func yamlToJSON(yamlData []byte) ([]byte, error) {
	var raw any
	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return nil, err
	}
	return json.Marshal(normaliseYAML(raw))
}

func normaliseYAML(v any) any {
	switch val := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, vv := range val {
			key, _ := k.(string)
			out[key] = normaliseYAML(vv)
		}
		return out
	case map[string]interface{}:
		for k, vv := range val {
			val[k] = normaliseYAML(vv)
		}
		return val
	case []interface{}:
		for i, item := range val {
			val[i] = normaliseYAML(item)
		}
		return val
	default:
		return v
	}
}
