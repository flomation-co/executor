// Package file_list lists the contents of a directory in the flow's workspace.
package file_list

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Files"
	Description  = "List the files and folders in the flow's workspace, optionally filtered or recursive."
	Summary      = "List files in the workspace"
	Website      = "https://www.flomation.co"
	Icon         = "file+list"
	Date         = "14/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "directory",
		Type:        core.ConnectionTypeString,
		Label:       "Folder to list, relative to the flow's workspace. Leave blank for the workspace itself.",
		Placeholder: "reports",
	},
	{
		Name:        "pattern",
		Type:        core.ConnectionTypeString,
		Label:       "Only include names matching this pattern (for example *.csv)",
		Placeholder: "*.csv",
	},
	{
		Name:  "recursive",
		Type:  core.ConnectionTypeBoolean,
		Label: "Include everything in sub-folders too",
	},
	{
		Name:  "include_directories",
		Type:  core.ConnectionTypeBoolean,
		Label: "Include folders in the results as well as files",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "entries", Type: core.ConnectionTypeObject, Label: "Entries"},
	{Name: "paths", Type: core.ConnectionTypeObject, Label: "Paths only"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Result Was Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	dir := file_common.Rel(file_common.OptionalString("directory", inputs))
	pattern := file_common.OptionalString("pattern", inputs)
	recursive := file_common.OptionalBool("recursive", inputs)
	includeDirs := file_common.OptionalBool("include_directories", inputs)

	// Fail on a bad pattern rather than silently matching nothing, which reads
	// as "the folder is empty" and sends the caller looking in the wrong place.
	if pattern != "" {
		if _, err := path.Match(pattern, "probe"); err != nil {
			return file_common.ErrorResult(fmt.Sprintf("%q is not a valid pattern: %v", pattern, err)), nil
		}
	}

	root, err := file_common.Workspace()
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	defer func() { _ = root.Close() }()

	info, err := root.Stat(dir)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, dir).Error()), nil
	}
	if !info.IsDir() {
		return file_common.ErrorResult(fmt.Sprintf("%q is a file, not a folder — use Read File to read it", dir)), nil
	}

	limit := file_common.MaxListEntries
	if recursive {
		limit = file_common.MaxWalkEntries
	}

	entries, truncated, err := collect(root.FS(), dir, pattern, recursive, includeDirs, limit)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, dir).Error()), nil
	}

	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}

	where := dir
	if where == "." {
		where = "the workspace"
	}
	summary := fmt.Sprintf("%d item(s) in %s", len(entries), where)
	if truncated {
		summary += fmt.Sprintf(" (stopped at %d — narrow it with a pattern or a sub-folder)", limit)
	}
	// The listing goes IN tool_result: an agent given only a count has to call
	// again to learn anything.
	if encoded, err := json.Marshal(entries); err == nil && len(entries) > 0 {
		summary += ":\n" + string(encoded)
	}

	return file_common.OkResult(summary, map[string]interface{}{
		"entries":   entries,
		"paths":     paths,
		"count":     len(entries),
		"truncated": truncated,
	}), nil
}

// collect walks the confined fs.FS. Every path it yields has already been
// resolved through the root, so nothing here can reach outside the workspace.
func collect(fsys fs.FS, dir, pattern string, recursive, includeDirs bool, limit int) ([]file_common.Entry, bool, error) {
	var entries []file_common.Entry
	truncated := false

	keep := func(rel string, d fs.DirEntry) {
		if d.IsDir() && !includeDirs {
			return
		}
		if pattern != "" {
			// Match on the base name, which is what someone means by "*.csv".
			if ok, _ := path.Match(pattern, path.Base(rel)); !ok {
				return
			}
		}
		info, _ := d.Info()
		entries = append(entries, file_common.EntryFrom(rel, info))
	}

	if !recursive {
		items, err := fs.ReadDir(fsys, dir)
		if err != nil {
			return nil, false, err
		}
		for _, d := range items {
			if len(entries) >= limit {
				truncated = true
				break
			}
			keep(joinRel(dir, d.Name()), d)
		}
		sortEntries(entries)
		return entries, truncated, nil
	}

	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// One unreadable sub-tree must not cost the caller the whole walk.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p == dir {
			return nil
		}
		if len(entries) >= limit {
			truncated = true
			return fs.SkipAll
		}
		keep(p, d)
		return nil
	})
	if err != nil {
		return nil, truncated, err
	}
	sortEntries(entries)
	return entries, truncated, nil
}

func joinRel(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}

// Folders first, then alphabetical — the order a person scanning a listing
// expects, and stable so a flow comparing two runs sees a real difference.
func sortEntries(entries []file_common.Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Path) < strings.ToLower(entries[j].Path)
	})
}
