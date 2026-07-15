// Package to_pdf combines one or more images into a single PDF. ImageMagick-backed.
package to_pdf

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	core "flomation.app/automate/executor"
	ic "flomation.app/automate/executor/actions/image"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Images to PDF"
	Description  = "Combine one or more images into a single PDF, one image per page"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "images", Type: core.ConnectionTypeText, Label: "Image references (one per line, in order)", Placeholder: "flo:file:…\nflo:blob:…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pdf", Type: core.ConnectionTypeString, Label: "PDF (media reference)"},
	{Name: "page_count", Type: core.ConnectionTypeInteger, Label: "Pages"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

var listSplit = regexp.MustCompile(`[\r\n,]+`)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	raw, err := ic.RequiredString("images", inputs)
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	var paths []string
	for _, part := range listSplit.Split(raw, -1) {
		if p := strings.TrimSpace(part); p != "" {
			local, _, e := flow.ResolveToLocalFile(p)
			if e != nil {
				return ic.ErrResult("could not read an image: " + e.Error())
			}
			paths = append(paths, local)
		}
	}
	if len(paths) == 0 {
		return ic.ErrResult("provide at least one image reference")
	}

	outPath, err := flow.MediaScratchFile("pdf")
	if err != nil {
		return ic.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), ic.DefaultTimeout)
	defer cancel()

	// magick <img> <img> … <out>.pdf → one page per image.
	args := append(append([]string{}, paths...), outPath)
	stderr, err := ic.RunMagick(ctx, args...)
	if err != nil {
		return ic.ErrResult(fmt.Sprintf("magick failed: %v: %s", err, ic.Tail(stderr, 400)))
	}

	ref, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return ic.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Built a %d-page PDF (%d bytes).", len(paths), size),
		"pdf":         ref,
		"page_count":  len(paths),
		"size_bytes":  size,
		"success":     true,
	}, nil
}
