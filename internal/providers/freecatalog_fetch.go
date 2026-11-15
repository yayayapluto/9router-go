package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
)

// freeCatalogLimit bounds the catalogue body: upstream /models listings stay
// well under a megabyte, so 2 MiB is generous and keeps a hostile or broken
// upstream from exhausting memory.
const freeCatalogLimit = 2 << 20

// FetchFreeModels returns the chat ids a keyless provider currently
// advertises, or nil when the catalogue cannot be read (the caller then keeps
// its static ProviderModels list). providerID must be a ProviderRegistry key
// with a ModelsFetcher.
//
// No timeout is applied here: the deadline belongs to the caller's ctx.
func FetchFreeModels(ctx context.Context, client *http.Client, providerID string) []string {
	meta, ok := ProviderRegistry[providerID]
	if !ok || meta.ModelsFetcher == nil || meta.ModelsFetcher.URL == "" {
		return nil
	}
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.ModelsFetcher.URL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	// LLM7 answers only to the literal anonymous token; every other keyless
	// upstream rejects an Authorization header it did not hand out.
	if providerID == "llm7-free" {
		req.Header.Set("Authorization", "Bearer unused")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, freeCatalogLimit))
	if err != nil {
		return nil
	}
	var catalog struct {
		Data   []map[string]any `json:"data"`
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil
	}
	entries := catalog.Data
	if len(entries) == 0 {
		entries = catalog.Models
	}

	seen := make(map[string]bool, len(entries))
	var out []string
	for _, entry := range entries {
		id, _ := entry["id"].(string)
		if id == "" || seen[id] || !freeCatalogKeeps(providerID, id, entry) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// freeCatalogKeeps applies the per-provider catalogue filter. Unknown
// providers keep everything: their catalogue is already the free lane.
func freeCatalogKeeps(providerID, id string, entry map[string]any) bool {
	switch providerID {
	case "kilo-free":
		// Kilo mixes paid models into the same catalogue; the free lane is
		// the `:free` suffix plus the ids the static registry already lists.
		return strings.HasSuffix(id, ":free") || slices.Contains(ProviderModels["kilo-free"], id)
	case "llm7-free":
		if tier, ok := entry["tier"].(string); ok && tier == "pro" {
			return false
		}
		if usageBased, ok := entry["usage_based_only"].(bool); ok && usageBased {
			return false
		}
		return true
	default:
		return true
	}
}
