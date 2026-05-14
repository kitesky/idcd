package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPIHandler_StatusOK(t *testing.T) {
	h := NewOpenAPIHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr := httptest.NewRecorder()

	h.OpenAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestOpenAPIHandler_ContentTypeJSON(t *testing.T) {
	h := NewOpenAPIHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr := httptest.NewRecorder()

	h.OpenAPI(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestOpenAPIHandler_ValidJSON(t *testing.T) {
	h := NewOpenAPIHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr := httptest.NewRecorder()

	h.OpenAPI(rr, req)

	var parsed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rr.Body.String())
	}
}

func TestOpenAPIHandler_ContainsOpenAPIField(t *testing.T) {
	h := NewOpenAPIHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr := httptest.NewRecorder()

	h.OpenAPI(rr, req)

	var parsed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if _, ok := parsed["openapi"]; !ok {
		t.Error("expected JSON to contain 'openapi' field")
	}
}

func TestOpenAPIHandler_ContainsPaths(t *testing.T) {
	h := NewOpenAPIHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr := httptest.NewRecorder()

	h.OpenAPI(rr, req)

	var parsed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	paths, ok := parsed["paths"]
	if !ok {
		t.Fatal("expected JSON to contain 'paths' field")
	}

	pathsMap, ok := paths.(map[string]any)
	if !ok {
		t.Fatalf("expected 'paths' to be an object, got %T", paths)
	}

	expectedPaths := []string{
		"/probe/http",
		"/probe/ping",
		"/probe/dns",
		"/probe/tcp",
		"/info/ip",
		"/info/whois",
		"/info/ssl",
		"/info/dns",
		"/info/icp",
		"/monitors",
		"/auth/login",
		"/auth/register",
	}

	for _, p := range expectedPaths {
		if _, exists := pathsMap[p]; !exists {
			t.Errorf("expected path %q to be present in spec", p)
		}
	}
}

func TestOpenAPIHandler_ContainsSecuritySchemes(t *testing.T) {
	h := NewOpenAPIHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr := httptest.NewRecorder()

	h.OpenAPI(rr, req)

	var parsed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	components, ok := parsed["components"].(map[string]any)
	if !ok {
		t.Fatal("expected 'components' to be present and be an object")
	}

	secSchemes, ok := components["securitySchemes"].(map[string]any)
	if !ok {
		t.Fatal("expected 'components.securitySchemes' to be present and be an object")
	}

	if _, ok := secSchemes["BearerAuth"]; !ok {
		t.Error("expected BearerAuth security scheme")
	}
	if _, ok := secSchemes["ApiKeyHeader"]; !ok {
		t.Error("expected ApiKeyHeader security scheme")
	}
}

func TestOpenAPIHandler_CachedResponse(t *testing.T) {
	// Verify the handler caches the result (call twice, should return same bytes)
	h := NewOpenAPIHandler()

	req1 := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr1 := httptest.NewRecorder()
	h.OpenAPI(rr1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr2 := httptest.NewRecorder()
	h.OpenAPI(rr2, req2)

	if rr1.Code != rr2.Code {
		t.Errorf("expected same status code on repeated calls, got %d vs %d", rr1.Code, rr2.Code)
	}
	if rr1.Body.String() != rr2.Body.String() {
		t.Error("expected identical response body on repeated calls")
	}
}

func TestOpenAPIHandler_InfoTitle(t *testing.T) {
	h := NewOpenAPIHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	rr := httptest.NewRecorder()

	h.OpenAPI(rr, req)

	var parsed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	info, ok := parsed["info"].(map[string]any)
	if !ok {
		t.Fatal("expected 'info' to be present and be an object")
	}

	title, ok := info["title"].(string)
	if !ok || title == "" {
		t.Error("expected non-empty 'info.title'")
	}
}

func TestYamlToJSON_ValidOutput(t *testing.T) {
	input := []byte(`
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths:
  /test:
    get:
      summary: Test endpoint
`)
	out, err := yamlToJSON(input)
	if err != nil {
		t.Fatalf("yamlToJSON returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["openapi"] != "3.1.0" {
		t.Errorf("expected openapi=3.1.0, got %v", parsed["openapi"])
	}
}

func TestNormaliseYAML_MapConversion(t *testing.T) {
	// Simulate what yaml.v2 produces: map[interface{}]interface{}
	input := map[interface{}]interface{}{
		"key1": "value1",
		"key2": map[interface{}]interface{}{
			"nested": "value",
		},
		"arr": []interface{}{
			map[interface{}]interface{}{"item": 1},
		},
	}

	result := normaliseYAML(input)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}
	if m["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", m["key1"])
	}
	nested, ok := m["key2"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested to be map[string]interface{}, got %T", m["key2"])
	}
	if nested["nested"] != "value" {
		t.Errorf("expected nested.nested=value, got %v", nested["nested"])
	}
}
