package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"9router/proxy/internal/db"
	"9router/proxy/internal/routing/freepool"
)

// freePoolFixture seeds two providers pointing at local servers and one combo
// with the "free" strategy, so a request to the combo walks the free pool.
// The primary server always answers 429; the fallback server always answers.
func freePoolFixture(t *testing.T, models []string) (h *ChatHandler, primaryHits, fallbackHits *atomic.Int32, teardown func()) {
	t.Helper()

	primaryHits, fallbackHits = new(atomic.Int32), new(atomic.Int32)
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limited, retry in 30s"}}`))
	}))
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-fallback",
			"object":  "chat.completion",
			"model":   "deepseek-chat",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "fallback answered"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))

	database, cleanup := setupChatTestDB(t)
	database.Exec(`DELETE FROM providerConnections`)
	database.Exec(`DELETE FROM combos`)

	dataPrimary, _ := json.Marshal(map[string]any{"apiKey": "sk-mock", "baseUrl": primaryServer.URL})
	database.Exec(`INSERT INTO providerConnections (id, provider, authType, name, priority, isActive, data, createdAt, updatedAt) VALUES ('conn-free-a', 'openai', 'apikey', 'a', 0, 1, ?, '2026-07-18T00:00:00Z', '2026-07-18T00:00:00Z')`, string(dataPrimary))
	dataFallback, _ := json.Marshal(map[string]any{"apiKey": "sk-mock", "baseUrl": fallbackServer.URL})
	database.Exec(`INSERT INTO providerConnections (id, provider, authType, name, priority, isActive, data, createdAt, updatedAt) VALUES ('conn-free-b', 'deepseek', 'apikey', 'b', 0, 1, ?, '2026-07-18T00:00:00Z', '2026-07-18T00:00:00Z')`, string(dataFallback))

	members, _ := json.Marshal(models)
	database.Exec(`INSERT INTO combos (id, name, kind, models, createdAt, updatedAt) VALUES ('cfree', 'free-pool', 'fallback', ?, '2026-07-18T00:00:00Z', '2026-07-18T00:00:00Z')`, string(members))

	repo := db.NewRepo(database)
	if err := repo.SetComboStrategy("free-pool", db.ComboStrategy{Strategy: "free"}); err != nil {
		t.Fatalf("SetComboStrategy: %v", err)
	}

	freepool.Default().Reset()
	return NewChatHandler(repo), primaryHits, fallbackHits, func() {
		freepool.Default().Reset()
		primaryServer.Close()
		fallbackServer.Close()
		cleanup()
	}
}

func postCombo(t *testing.T, h *ChatHandler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"free-pool","messages":[{"role":"user","content":"hi"}],"stream":false}`))
	rec := httptest.NewRecorder()
	h.HandleChatCompletions(rec, req)
	return rec
}

// TestFreePoolSkipsParkedModel proves a parked member is never dialled: the
// pool drops it before the combo loop picks a candidate.
func TestFreePoolSkipsParkedModel(t *testing.T) {
	h, primaryHits, fallbackHits, teardown := freePoolFixture(t, []string{"openai/gpt-4o", "deepseek/deepseek-chat"})
	defer teardown()

	freepool.Default().Park("openai", "gpt-4o", time.Minute)

	rec := postCombo(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "fallback answered") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
	if got := fallbackHits.Load(); got != 1 {
		t.Errorf("expected the fallback server to serve the turn, got %d hits", got)
	}
	if got := primaryHits.Load(); got != 0 {
		t.Errorf("expected the parked model to receive zero requests, got %d", got)
	}
}

// TestFreePoolParksRateLimitedModel proves a retryable failure parks the
// failing model for at least the wait the upstream announced.
func TestFreePoolParksRateLimitedModel(t *testing.T) {
	h, primaryHits, fallbackHits, teardown := freePoolFixture(t, []string{"openai/gpt-4o", "deepseek/deepseek-chat"})
	defer teardown()

	rec := postCombo(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from the fallback member, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "fallback answered") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
	if !freepool.Default().Parked("openai", "gpt-4o") {
		t.Fatal("429 did not park openai/gpt-4o in the free pool")
	}
	if got := primaryHits.Load(); got != 1 {
		t.Errorf("expected exactly one request to the 429 model, got %d", got)
	}

	// Second request: the pool keeps the parked model out.
	rec = postCombo(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("second request expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fallbackHits.Load(); got != 2 {
		t.Errorf("expected the fallback server to serve the second turn, got %d hits", got)
	}
	if got := primaryHits.Load(); got != 1 {
		t.Errorf("parked model received another request: %d total", got)
	}
}
