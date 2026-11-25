package provider

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLegacyPartnerPreset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partners.json")
	data := `{"seen": {
		"acme": {"id":"acme","name":"Acme","kind":"partner","sponsored":true,"noKey":true,"decide":"x","chat":"https://acme.example/v1","anthropic":"https://acme.example","searches":true,"catalog":"acme","regions":[{"id":"global","chat":"https://global.acme.example/v1","decide":"x"}]},
		"regional": {"id":"regional","name":"Regional","regions":[{"id":"global","chat":"https://regional.example/v1","catalog":"region"}]},
		"plug": {"id":"plug","name":"Plugin","chat":"https://plug.example/v1","plugin":"sample-plugin"},
		"unsafe": {"id":"unsafe","name":"Unsafe","chat":"http://unsafe.example/v1"},
		"openai": {"id":"openai","name":"Shadow","chat":"https://shadow.example/v1"},
		"mismatch": {"id":"other","name":"Mismatch","chat":"https://other.example/v1"}
	}, "partners":[{"id":"new","name":"New","chat":"https://new.example/v1"}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	kept := readLegacyPresets(path)
	if len(kept) != 2 {
		t.Fatalf("want only two cached provider presets, got %v", kept)
	}
	preset := kept["acme"]
	if preset == nil || preset.Chat != "https://acme.example/v1" || preset.Catalog != "acme" || preset.Kind != KindPartner || preset.Sponsored || preset.NoKey || preset.Decide != "" || len(preset.Regions) != 1 || preset.Regions[0].Decide != "" {
		t.Fatalf("legacy preset lost its endpoints or retained promotion fields: %+v", preset)
	}
	if regional := kept["regional"]; regional == nil || regional.Chat != "https://regional.example/v1" || regional.Catalog != "region" {
		t.Fatalf("legacy preset lost its default region: %+v", regional)
	}
	if slices.ContainsFunc(Presets(), func(preset PresetDef) bool { return preset.Kind == KindPartner }) {
		t.Fatal("legacy partners are listed as new provider presets")
	}
	original := legacyPresets
	legacyPresets = func() map[string]*PresetDef { return kept }
	t.Cleanup(func() { legacyPresets = original })
	saved := Provider{Preset: "acme", Anthropic: "https://acme.example"}
	if !PresetSearches(saved) {
		t.Fatal("the cached preset's search capability was lost")
	}
	saved.Anthropic = "https://elsewhere.example"
	if PresetSearches(saved) {
		t.Fatal("the cached preset's search capability followed to a different host")
	}
}
