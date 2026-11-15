package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// withFreeCatalog points providerID's ModelsFetcher at url for the test.
func withFreeCatalog(t *testing.T, providerID, url string) {
	t.Helper()
	prev, ok := ProviderRegistry[providerID]
	if !ok {
		t.Fatalf("provider %q missing from ProviderRegistry", providerID)
	}
	meta := prev
	meta.ModelsFetcher = &Fetcher{URL: url}
	ProviderRegistry[providerID] = meta
	t.Cleanup(func() { ProviderRegistry[providerID] = prev })
}

func TestFetchFreeModels(t *testing.T) {
	body := `{"data":[{"id":"a-free"}]}`
	var gotAuth string
	var gotAccept string
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Run("llm7-free sends anonymous token and drops paid tiers", func(t *testing.T) {
		withFreeCatalog(t, "llm7-free", srv.URL)
		body = `{"data":[
			{"id":"gpt-oss:20b"},
			{"id":"paid-pro","tier":"pro"},
			{"id":"paid-usage","usage_based_only":true},
			{"id":"free-explicit","tier":"free","usage_based_only":false},
			{"id":"gpt-oss:20b"}
		]}`
		defer func() { body = `{"data":[{"id":"a-free"}]}` }()

		got := FetchFreeModels(context.Background(), srv.Client(), "llm7-free")
		if gotAuth != "Bearer unused" {
			t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer unused")
		}
		want := []string{"free-explicit", "gpt-oss:20b"}
		if !slices.Equal(got, want) {
			t.Errorf("models = %v, want %v", got, want)
		}
	})

	t.Run("ovh sends no auth header and keeps every id", func(t *testing.T) {
		withFreeCatalog(t, "ovh", srv.URL)
		body = `{"data":[{"id":"gpt-oss-120b"},{"id":"Qwen3.5-397B-A17B"},{"id":"billed-lookalike"}]}`
		defer func() { body = `{"data":[{"id":"a-free"}]}` }()

		got := FetchFreeModels(context.Background(), srv.Client(), "ovh")
		if gotAuth != "" {
			t.Errorf("Authorization = %q, want empty", gotAuth)
		}
		want := []string{"Qwen3.5-397B-A17B", "billed-lookalike", "gpt-oss-120b"}
		if !slices.Equal(got, want) {
			t.Errorf("models = %v, want %v", got, want)
		}
	})

	t.Run("kilo-free keeps :free and registry ids only", func(t *testing.T) {
		withFreeCatalog(t, "kilo-free", srv.URL)
		body = `{"models":[
			{"id":"kilo-auto/free"},
			{"id":"nvidia/nemotron-3.5-super-vl:free"},
			{"id":"openrouter/free"},
			{"id":"anthropic/claude-opus-4:paid"}
		]}`
		defer func() { body = `{"data":[{"id":"a-free"}]}` }()

		got := FetchFreeModels(context.Background(), srv.Client(), "kilo-free")
		want := []string{"kilo-auto/free", "nvidia/nemotron-3.5-super-vl:free", "openrouter/free"}
		if !slices.Equal(got, want) {
			t.Errorf("models = %v, want %v", got, want)
		}
	})

	t.Run("non-2xx returns nil", func(t *testing.T) {
		withFreeCatalog(t, "ovh", srv.URL)
		status = http.StatusInternalServerError
		defer func() { status = http.StatusOK }()

		if got := FetchFreeModels(context.Background(), srv.Client(), "ovh"); got != nil {
			t.Errorf("models = %v, want nil", got)
		}
	})

	t.Run("malformed json returns nil", func(t *testing.T) {
		withFreeCatalog(t, "ovh", srv.URL)
		body = `not json`
		defer func() { body = `{"data":[{"id":"a-free"}]}` }()

		if got := FetchFreeModels(context.Background(), srv.Client(), "ovh"); got != nil {
			t.Errorf("models = %v, want nil", got)
		}
	})

	t.Run("unknown provider returns nil", func(t *testing.T) {
		if got := FetchFreeModels(context.Background(), srv.Client(), "no-such-provider"); got != nil {
			t.Errorf("models = %v, want nil", got)
		}
	})

	t.Run("nil fetcher returns nil", func(t *testing.T) {
		prev := ProviderRegistry["ovh"]
		meta := prev
		meta.ModelsFetcher = nil
		ProviderRegistry["ovh"] = meta
		defer func() { ProviderRegistry["ovh"] = prev }()

		if got := FetchFreeModels(context.Background(), srv.Client(), "ovh"); got != nil {
			t.Errorf("models = %v, want nil", got)
		}
	})

	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
}
