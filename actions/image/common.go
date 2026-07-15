package image_common

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	DefaultTimeout = 120 * time.Second
	stderrCap      = 256 * 1024
)

// safetyLimits guard against decode bombs — a malformed/huge image cannot exhaust
// the runner. Prepended to every magick invocation (they must precede the input).
var safetyLimits = []string{
	"-limit", "memory", "512MiB",
	"-limit", "map", "1GiB",
	"-limit", "disk", "2GiB",
	"-limit", "time", "120",
}

// MagickPath resolves the ImageMagick CLI: FLOMATION_MAGICK_PATH override, then
// `magick` (v7), then `convert` (v6). No runtime download.
func MagickPath() (string, error) {
	if p := os.Getenv("FLOMATION_MAGICK_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("FLOMATION_MAGICK_PATH=%q does not exist", p)
	}
	if p, err := exec.LookPath("magick"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("convert"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("ImageMagick (magick/convert) not found on PATH — install it on the runner host")
}

type limitedBuffer struct {
	buf bytes.Buffer
	cap int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if remaining := l.cap - l.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			l.buf.Write(p[:remaining])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string { return l.buf.String() }

// RunMagick runs the ImageMagick CLI with the given operation args (NO shell —
// every arg is action-controlled). Safety limits are prepended automatically.
func RunMagick(ctx context.Context, args ...string) (string, error) {
	bin, err := MagickPath()
	if err != nil {
		return "", err
	}
	full := append(append([]string{}, safetyLimits...), args...)
	// #nosec G204 -- args are built from typed inputs, never a raw command string.
	cmd := exec.CommandContext(ctx, bin, full...)
	stderr := &limitedBuffer{cap: stderrCap}
	cmd.Stderr = stderr
	err = cmd.Run()
	return stderr.String(), err
}

// ImageInfo is the subset of `identify` output the actions surface.
type ImageInfo struct {
	Width  int
	Height int
	Format string
}

// Identify reads an image's dimensions + format. Uses `magick identify` (v7) or
// the standalone `identify` (v6).
func Identify(ctx context.Context, path string) (*ImageInfo, error) {
	var bin string
	var pre []string
	if p := os.Getenv("FLOMATION_MAGICK_PATH"); p != "" {
		bin, pre = p, []string{"identify"}
	} else if p, err := exec.LookPath("magick"); err == nil {
		bin, pre = p, []string{"identify"}
	} else if p, err := exec.LookPath("identify"); err == nil {
		bin = p
	} else {
		return nil, fmt.Errorf("ImageMagick identify not found on PATH — install it on the runner host")
	}

	// %w %h %m of the first frame ([0]) — a single line even for multi-frame images.
	args := append(append([]string{}, pre...), "-format", "%w %h %m", path+"[0]")
	// #nosec G204 -- path is a workspace-confined file, args are constants.
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("identify failed: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 3 {
		return nil, fmt.Errorf("unexpected identify output: %q", string(out))
	}
	w, _ := strconv.Atoi(fields[0])
	h, _ := strconv.Atoi(fields[1])
	return &ImageInfo{Width: w, Height: h, Format: fields[2]}, nil
}

// ── Input helpers (kept local so the image category has no dependency on video) ──

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

func OptionalStringDefault(name, def string, inputs []*core.Connection) string {
	if v := OptionalString(name, inputs); v != "" {
		return v
	}
	return def
}

func OptionalInt(name string, def int, inputs []*core.Connection) int {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	switch v := c.Value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func OptionalBool(name string, def bool, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	if v, ok := c.Value.(bool); ok {
		return v
	}
	if s, ok := c.Value.(string); ok {
		return strings.EqualFold(strings.TrimSpace(s), "true")
	}
	return def
}

func ErrResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"success":     false,
		"error":       msg,
	}, nil
}

func Tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
