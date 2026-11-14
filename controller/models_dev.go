package controller

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/sync/singleflight"
)

// modelsDevCapabilities contains model metadata only; catalog prices are not imported.
// Returned catalog entries and their slices are immutable after publication.
type modelsDevCapabilities struct {
	ID               string `json:"id"`
	CanonicalModelID string `json:"canonical_model_id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Reasoning        bool   `json:"reasoning"`
	ToolCall         bool   `json:"tool_call"`
	ReasoningOptions []struct {
		Type   string   `json:"type"`
		Values []string `json:"values"`
	} `json:"reasoning_options"`
	Modalities struct {
		Input []string `json:"input"`
	} `json:"modalities"`
	Limit struct {
		Context int64 `json:"context"`
		Input   int64 `json:"input"`
	} `json:"limit"`
	native bool
}

type modelsDevCapabilityCache struct {
	sync.RWMutex
	models map[string]modelsDevCapabilities
	fetch  singleflight.Group
}

var modelsDevCatalog = &modelsDevCapabilityCache{}

// getModelsDevCapabilities fetches once on demand and retains the successful
// catalog until process exit. Failed fetches are not cached.
func getModelsDevCapabilities(ctx context.Context) (map[string]modelsDevCapabilities, error) {
	modelsDevCatalog.RLock()
	models := modelsDevCatalog.models
	modelsDevCatalog.RUnlock()
	if models != nil {
		return models, nil
	}

	result, err, _ := modelsDevCatalog.fetch.Do("catalog", func() (any, error) {
		modelsDevCatalog.RLock()
		models := modelsDevCatalog.models
		modelsDevCatalog.RUnlock()
		if models != nil {
			return models, nil
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://models.dev/api.json", nil)
		if err != nil {
			return nil, err
		}
		client := getHTTPClient()
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("models.dev returned HTTP %d", resp.StatusCode)
		}

		var providers map[string]struct {
			Models map[string]modelsDevCapabilities `json:"models"`
		}
		if err := common.DecodeJson(io.LimitReader(resp.Body, 32<<20), &providers); err != nil {
			return nil, fmt.Errorf("decode models.dev capabilities: %w", err)
		}
		models = make(map[string]modelsDevCapabilities)
		// Stable ordering resolves duplicate third-party offerings consistently;
		// a model's native offering takes precedence over reseller metadata.
		for _, providerID := range slices.Sorted(maps.Keys(providers)) {
			provider := providers[providerID]
			for _, modelID := range slices.Sorted(maps.Keys(provider.Models)) {
				entry := provider.Models[modelID]
				entry.native = entry.CanonicalModelID == "" || strings.HasPrefix(entry.CanonicalModelID, providerID+"/")
				_, canonicalSlug, _ := strings.Cut(entry.CanonicalModelID, "/")
				for _, key := range [...]string{modelID, entry.ID, entry.CanonicalModelID, canonicalSlug} {
					if key == "" {
						continue
					}
					current, exists := models[key]
					if !exists || entry.native && !current.native {
						models[key] = entry
					}
				}
				models[providerID+"/"+modelID] = entry
			}
		}
		if len(models) == 0 {
			return nil, fmt.Errorf("models.dev returned no model capabilities")
		}

		modelsDevCatalog.Lock()
		modelsDevCatalog.models = models
		modelsDevCatalog.Unlock()
		return models, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(map[string]modelsDevCapabilities), nil
}
