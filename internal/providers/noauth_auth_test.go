package providers

import (
	"slices"
	"strings"
	"testing"
)

// Every keyless provider the combo path may dial needs an auth token it can
// hand to the forwarder. A no-auth entry with neither a DefaultAPIKey nor a
// static Authorization header makes the failover loop answer 401 "no API key
// found" before the request ever leaves the gateway — the exact shape the
// free-tier combo hit when ovh and kilo-free were registered without one.
func TestNoAuthProvidersCarryAForwardableToken(t *testing.T) {
	for id, meta := range ProviderRegistry {
		// The chat lane only: media-only keyless providers authenticate through
		// their own executors, not the combo forwarder.
		if !meta.NoAuth || meta.Hidden || !slices.Contains(meta.ServiceKinds, "llm") {
			continue
		}
		cfg, ok := KnownProviders[id]
		if !ok {
			t.Errorf("%s: no KnownProviders entry", id)
			continue
		}
		// A local lane (devin://acp/stdio) never goes through the HTTP
		// forwarder, so it needs no token at all.
		if !strings.HasPrefix(cfg.BaseURL, "http") {
			continue
		}
		if cfg.DefaultAPIKey == "" && cfg.StaticHeaders["Authorization"] == "" {
			t.Errorf("%s: no DefaultAPIKey and no static Authorization header", id)
		}
	}
}
