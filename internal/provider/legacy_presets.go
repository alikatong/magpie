package provider

import (
	"cmp"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/lazyre"
)

const KindPartner Kind = "partner"

var legacyPresetID = lazyre.New(`^[a-z0-9][a-z0-9-]{1,40}$`)

var legacyPresets = sync.OnceValue(func() map[string]*PresetDef {
	return readLegacyPresets(filepath.Join(appdir.Cache(), "partners.json"))
})

func readLegacyPresets(path string) map[string]*PresetDef {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache struct {
		Seen map[string]struct {
			PresetDef
			Plugin string `json:"plugin"`
		} `json:"seen"`
	}
	if json.Unmarshal(data, &cache) != nil {
		return nil
	}
	result := make(map[string]*PresetDef, len(cache.Seen))
	for id, entry := range cache.Seen {
		preset := entry.PresetDef
		if id != preset.ID || !legacyPresetID.MatchString(id) || builtinPreset(id) != nil || entry.Plugin != "" || strings.TrimSpace(preset.Name) == "" {
			continue
		}
		if !validLegacyEndpoints(preset) {
			continue
		}
		preset.Kind, preset.Sponsored = KindPartner, false
		preset.Decide, preset.NoKey, preset.Hosts = "", false, false
		preset.Endpoint, preset.EndpointHint, preset.EndpointNeeded = "", "", ""
		for index := range preset.Regions {
			preset.Regions[index].Decide = ""
		}
		if preset.Chat == "" && preset.Responses == "" && preset.Anthropic == "" {
			region := preset.Regions[0]
			preset.Chat, preset.Responses, preset.Anthropic = region.Chat, region.Responses, region.Anthropic
			preset.KeysURL = cmp.Or(region.KeysURL, preset.KeysURL)
			preset.Website = cmp.Or(region.Website, preset.Website)
			preset.Catalog = cmp.Or(region.Catalog, preset.Catalog)
			if region.Website != "" {
				preset.SiteHost = region.SiteHost
			}
		}
		if name, ok := strings.CutPrefix(preset.Icon, iconPrefix); !ok || IconFile(name) == "" {
			preset.Icon = "generic"
		}
		result[id] = &preset
	}
	return result
}

func validLegacyEndpoints(preset PresetDef) bool {
	if preset.Chat == "" && preset.Responses == "" && preset.Anthropic == "" && len(preset.Regions) == 0 {
		return false
	}
	urls := []string{preset.Chat, preset.Responses, preset.Anthropic, preset.Website, preset.KeysURL}
	for _, region := range preset.Regions {
		if region.Chat == "" && region.Responses == "" && region.Anthropic == "" {
			return false
		}
		urls = append(urls, region.Chat, region.Responses, region.Anthropic, region.Website, region.KeysURL)
	}
	return !slices.ContainsFunc(urls, func(raw string) bool {
		if raw == "" {
			return false
		}
		parsed, err := url.Parse(raw)
		return err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil
	})
}
