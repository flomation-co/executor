// Package iconlint validates that every icon referenced by an action (or its
// category) resolves in the editor's icon set.
//
// Action icons live in Go metadata (each action's Icon constant) and flow into
// the manifest, but whether an icon actually renders is decided by the editor's
// icon registry (automate/editor/app/components/icons/paths.ts). Nothing in Go
// cross-checks the two, so an invalid icon name compiles cleanly and only shows
// up as a "?" placeholder in the UI. This package closes that gap by validating
// the manifest's icon names against a vendored snapshot of the editor's set
// (valid_icons.txt); the accompanying test fails CI on any invalid icon.
package iconlint

import (
	_ "embed"
	"strings"
)

//go:embed valid_icons.txt
var validIconsRaw string

// ValidIcons returns the set of icon names the editor can render, parsed from
// the vendored valid_icons.txt (lines beginning with '#' are comments).
func ValidIcons() map[string]bool {
	set := make(map[string]bool)
	for _, line := range strings.Split(validIconsRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[line] = true
	}
	return set
}

// InvalidParts returns the parts of a composite "base+badge" icon string that
// are not in valid. An empty (nil) result means the icon fully resolves. An
// empty icon string is treated as valid (no icon is a legitimate choice).
func InvalidParts(icon string, valid map[string]bool) []string {
	var bad []string
	for _, part := range strings.Split(icon, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !valid[part] {
			bad = append(bad, part)
		}
	}
	return bad
}
