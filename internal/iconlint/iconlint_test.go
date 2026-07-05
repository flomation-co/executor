package iconlint

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// manifestRelPath is the committed manifest, relative to this package directory.
const manifestRelPath = "../assets/manifest/manifest.json"

// TestInvalidParts proves the validator actually rejects bad names (guards
// against a lint that silently passes everything).
func TestInvalidParts(t *testing.T) {
	valid := ValidIcons()
	if !valid["shield-halved"] || !valid["magnifying-glass"] {
		t.Fatal("expected known-good names to be present in valid set")
	}
	cases := []struct {
		icon string
		bad  []string
	}{
		{"shield-halved", nil},             // valid single
		{"map+circle-check", nil},          // valid composite
		{"", nil},                          // no icon is fine
		{"building", []string{"building"}}, // invalid single (not in set)
		{"briefcase+car", []string{"car"}}, // valid base, invalid badge
		{"utensils+location-dot", []string{"utensils", "location-dot"}}, // both invalid
	}
	for _, c := range cases {
		got := InvalidParts(c.icon, valid)
		if len(got) != len(c.bad) {
			t.Errorf("InvalidParts(%q) = %v, want %v", c.icon, got, c.bad)
			continue
		}
		for i := range got {
			if got[i] != c.bad[i] {
				t.Errorf("InvalidParts(%q) = %v, want %v", c.icon, got, c.bad)
				break
			}
		}
	}
}

type manifestEntry struct {
	Icon     string `json:"icon"`
	Category *struct {
		Icon string `json:"icon"`
	} `json:"category"`
	SubCategory *struct {
		Icon string `json:"icon"`
	} `json:"sub_category"`
}

// TestManifestIconsResolve fails if any action, category, or sub-category icon
// in the committed manifest is not in the editor's icon set. This is the guard
// that keeps the manifest ↔ paths.ts icon contract honest: without it, an
// invalid icon name compiles and ships as a "?" placeholder unnoticed.
//
// To fix a failure: pick a valid name from internal/iconlint/valid_icons.txt
// (each half of a "base+badge" composite must be valid), or add the icon to the
// editor's paths.ts and regenerate valid_icons.txt (see its header).
func TestManifestIconsResolve(t *testing.T) {
	valid := ValidIcons()
	if len(valid) == 0 {
		t.Fatal("no valid icon names loaded from valid_icons.txt")
	}

	data, err := os.ReadFile(manifestRelPath)
	if err != nil {
		t.Fatalf("read manifest %s: %v", manifestRelPath, err)
	}
	var manifest map[string]manifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	type failure struct{ action, field, icon, part string }
	var fails []failure
	for id, e := range manifest {
		check := func(field, icon string) {
			for _, part := range InvalidParts(icon, valid) {
				fails = append(fails, failure{id, field, icon, part})
			}
		}
		check("icon", e.Icon)
		if e.Category != nil {
			check("category.icon", e.Category.Icon)
		}
		if e.SubCategory != nil {
			check("sub_category.icon", e.SubCategory.Icon)
		}
	}

	if len(fails) > 0 {
		sort.Slice(fails, func(i, j int) bool { return fails[i].action < fails[j].action })
		for _, f := range fails {
			t.Errorf("%s %s=%q: icon part %q is not in the editor icon set", f.action, f.field, f.icon, f.part)
		}
		t.Errorf("%d invalid icon usage(s) — pick valid names from internal/iconlint/valid_icons.txt (or add to the editor's paths.ts and regenerate)", len(fails))
	}
}
