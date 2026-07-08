package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// A self-similar wago.json: module-level engines/platforms are inherited by the
// subpackage that omits them; the subpackage with its own engines keeps them.
const sampleManifest = `{
  "schema": "wago-plugin/v1",
  "module": "github.com/wago-org/wasi",
  "license": "Apache-2.0",
  "engines": { "wago": ">=0.1.0", "tinygo": "*" },
  "platforms": ["linux/amd64"],
  "subpackages": [
    {
      "module": "github.com/wago-org/wasi/p1",
      "name": "WASI preview 1",
      "version": "1.0.0",
      "description": "preview1",
      "stability": "stable",
      "keywords": ["wasi", "wasi-preview1"]
    },
    {
      "module": "github.com/wago-org/wasi/p2",
      "name": "WASI preview 2",
      "stability": "experimental",
      "keywords": ["wasi", "component-model"],
      "engines": { "wago": ">=0.1.0" }
    }
  ]
}`

func TestManifestResolvedSubpackages(t *testing.T) {
	var m Manifest
	if err := json.Unmarshal([]byte(sampleManifest), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Schema != "wago-plugin/v1" || m.Module != "github.com/wago-org/wasi" {
		t.Fatalf("top-level fields lost: schema=%q module=%q", m.Schema, m.Module)
	}
	subs := m.ResolvedSubpackages()
	if len(subs) != 2 {
		t.Fatalf("want 2 subpackages, got %d", len(subs))
	}

	p1 := subs[0]
	if p1.Import != "github.com/wago-org/wasi/p1" {
		t.Errorf("import should be the module path, got %q", p1.Import)
	}
	if p1.ID != "p1" {
		t.Errorf("id should be the last module segment, got %q", p1.ID)
	}
	if strings.Join(p1.Tags, ",") != "wasi,wasi-preview1" {
		t.Errorf("keywords should map to tags, got %v", p1.Tags)
	}
	// p1 omitted engines/platforms → inherit from the module.
	if p1.Compat.Engines["tinygo"] != "*" || p1.Compat.Engines["wago"] != ">=0.1.0" {
		t.Errorf("p1 should inherit module engines, got %v", p1.Compat.Engines)
	}
	if len(p1.Compat.Platforms) != 1 || p1.Compat.Platforms[0] != "linux/amd64" {
		t.Errorf("p1 should inherit module platforms, got %v", p1.Compat.Platforms)
	}

	p2 := subs[1]
	// p2 set its own engines (no tinygo) → not overridden; platforms inherited.
	if _, hasTiny := p2.Compat.Engines["tinygo"]; hasTiny {
		t.Errorf("p2 declared its own engines; must not inherit tinygo, got %v", p2.Compat.Engines)
	}
	if len(p2.Compat.Platforms) != 1 {
		t.Errorf("p2 should inherit module platforms, got %v", p2.Compat.Platforms)
	}
}

// A "./path/wago.json" string subpackage must be rejected (the server can't
// resolve it; the publisher inlines it first).
func TestManifestRejectsPathRef(t *testing.T) {
	const withPathRef = `{"schema":"wago-plugin/v1","module":"m","subpackages":["./p2/wago.json"]}`
	var m Manifest
	err := json.Unmarshal([]byte(withPathRef), &m)
	if err == nil {
		t.Fatal("expected an error for an unresolved path-ref subpackage")
	}
	if !strings.Contains(err.Error(), "inlined") {
		t.Errorf("error should explain path refs must be inlined, got: %v", err)
	}
}
