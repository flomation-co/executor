// Package list_targets parses a Makefile and returns all available targets
// with their descriptions (extracted from comments preceding the target).
package list_targets

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Make Targets"
	Description  = "List all targets defined in a Makefile"
	Website      = "https://www.flomation.co"
	Icon         = "gears"
	Date         = "31/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "working_directory",
		Type:        core.ConnectionTypeString,
		Label:       "Working Directory",
		Placeholder: "/path/to/project",
	},
	{
		Name:        "makefile_path",
		Type:        core.ConnectionTypeString,
		Label:       "Makefile Path",
		Placeholder: "Makefile",
	},
}

// TargetInfo describes a single Makefile target.
type TargetInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Phony        bool     `json:"phony"`
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "targets", Type: core.ConnectionTypeString, Label: "Targets (JSON)"},
	{Name: "target_names", Type: core.ConnectionTypeString, Label: "Target Names (comma-separated)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Target Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	workDir := optStr("working_directory", inputs)
	makefilePath := optStr("makefile_path", inputs)

	if workDir == "" || strings.HasPrefix(workDir, "${") {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return errResult(fmt.Sprintf("unable to determine working directory: %v", err))
		}
	}

	// Resolve Makefile path
	if makefilePath == "" || strings.HasPrefix(makefilePath, "${") {
		makefilePath = findMakefile(workDir)
		if makefilePath == "" {
			return errResult("no Makefile found in working directory")
		}
	} else if !filepath.IsAbs(makefilePath) {
		makefilePath = filepath.Join(workDir, makefilePath)
	}

	targets, err := parseMakefile(makefilePath)
	if err != nil {
		return errResult(fmt.Sprintf("failed to parse Makefile: %v", err))
	}

	if len(targets) == 0 {
		return map[string]interface{}{
			"tool_result":  "No targets found in Makefile",
			"targets":      "[]",
			"target_names": "",
			"count":        int64(0),
			"success":      true,
		}, nil
	}

	targetsJSON, _ := json.Marshal(targets)
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.Name
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d target(s): %s", len(targets), strings.Join(names, ", ")),
		"targets":      string(targetsJSON),
		"target_names": strings.Join(names, ", "),
		"count":        int64(len(targets)),
		"success":      true,
	}, nil
}

// parseMakefile reads a Makefile and extracts targets, descriptions, and dependencies.
func parseMakefile(path string) ([]TargetInfo, error) {
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var targets []TargetInfo
	phonyTargets := make(map[string]bool)
	var pendingComment string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Track .PHONY declarations
		if strings.HasPrefix(line, ".PHONY") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				for _, name := range strings.Fields(parts[1]) {
					phonyTargets[name] = true
				}
			}
			continue
		}

		// Capture comment lines immediately before targets
		if strings.HasPrefix(line, "#") {
			comment := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			// Only keep the last comment line before a target
			pendingComment = comment
			continue
		}

		// Detect target lines: name: [dependencies]
		// Must start at column 0 (not indented) and contain a colon
		if len(line) > 0 && line[0] != '\t' && line[0] != ' ' && line[0] != '.' {
			colonIdx := strings.IndexByte(line, ':')
			if colonIdx > 0 {
				beforeColon := line[:colonIdx]

				// Skip variable assignments (=, :=, ?=, +=)
				if strings.ContainsAny(beforeColon, "=?+") {
					pendingComment = ""
					continue
				}

				// Skip immediate/conditional assignments where := appears
				afterAll := line[colonIdx:]
				if strings.HasPrefix(afterAll, ":=") || strings.HasPrefix(afterAll, "::=") {
					pendingComment = ""
					continue
				}

				// Skip lines with :: (double colon rules are unusual)
				afterColon := ""
				if colonIdx+1 < len(line) {
					if line[colonIdx+1] == ':' {
						pendingComment = ""
						continue
					}
					afterColon = strings.TrimSpace(line[colonIdx+1:])
				}

				targetName := strings.TrimSpace(beforeColon)

				// Skip targets with % (pattern rules)
				if strings.Contains(targetName, "%") {
					pendingComment = ""
					continue
				}

				// Parse dependencies
				var deps []string
				if afterColon != "" {
					for _, d := range strings.Fields(afterColon) {
						if !strings.HasPrefix(d, "#") {
							deps = append(deps, d)
						} else {
							break // Rest is a comment
						}
					}
				}

				targets = append(targets, TargetInfo{
					Name:         targetName,
					Description:  pendingComment,
					Dependencies: deps,
					Phony:        phonyTargets[targetName],
				})
			}
		}

		// Reset pending comment if this line isn't a comment or target
		if !strings.HasPrefix(line, "#") {
			pendingComment = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Makefile: %w", err)
	}

	// Apply phony flags to any targets found after .PHONY declarations
	for i := range targets {
		if phonyTargets[targets[i].Name] {
			targets[i].Phony = true
		}
	}

	return targets, nil
}

// findMakefile looks for common Makefile names in the given directory.
func findMakefile(dir string) string {
	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":  "Error: " + msg,
		"targets":      "[]",
		"target_names": "",
		"count":        int64(0),
		"success":      false,
	}, nil
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
