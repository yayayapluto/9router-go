package dashboard

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9router/proxy/internal/db"
)

// newNodeModelsNode stores one keyless custom node and returns its id.
func newNodeModelsNode(t *testing.T, repo *db.Repo, baseURL string) string {
	t.Helper()
	const id = "openai-compatible-chat-bansos"
	_, err := repo.CreateProviderNode(id, "openai-compatible", "Bansos", `{"prefix":"bansos","apiType":"chat","baseUrl":"`+baseURL+`"}`)
	if err != nil {
		t.Fatalf("CreateProviderNode: %v", err)
	}
	return id
}

// TestNodeModelsImportsWithoutAConnection: a keyless node has no connection row,
// so the node-level route is the only way "Import from /models" can work for it.
func TestNodeModelsImportsWithoutAConnection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("probed %q, want /v1/models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-oss-120b"},{"id":"qwen3-9b"}]}`))
	}))
	defer upstream.Close()

	repo, cleanup := setupTestDB(t)
	defer cleanup()
	id := newNodeModelsNode(t, repo, upstream.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/provider-nodes/"+id+"/models", nil)
	req.RemoteAddr = "127.0.0.1:3456"
	rec := httptest.NewRecorder()
	setupTestRouter(repo).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "gpt-oss-120b") || !strings.Contains(body, "qwen3-9b") {
		t.Errorf("catalogue not returned: %s", body)
	}
}

// TestNodeModelsOfflineNamesTheRefusal: a refused upstream must say so instead of
// echoing a raw `dial tcp … connectex` string, which is what the dashboard used
// to show for a node whose base URL pointed at a dead port.
func TestNodeModelsOfflineNamesTheRefusal(t *testing.T) {
	// Bind then close: the port is free and guaranteed to refuse.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	repo, cleanup := setupTestDB(t)
	defer cleanup()
	id := newNodeModelsNode(t, repo, "http://"+addr)

	req := httptest.NewRequest(http.MethodGet, "/api/provider-nodes/"+id+"/models", nil)
	req.RemoteAddr = "127.0.0.1:3456"
	rec := httptest.NewRecorder()
	setupTestRouter(repo).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Connection refused") {
		t.Errorf("refusal not named: %s", rec.Body.String())
	}
}

// TestNodeModelsUnknownIDIs404: a stale id in the dashboard must not probe an
// empty URL.
func TestNodeModelsUnknownIDIs404(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/provider-nodes/openai-compatible-chat-gone/models", nil)
	req.RemoteAddr = "127.0.0.1:3456"
	rec := httptest.NewRecorder()
	setupTestRouter(repo).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}
