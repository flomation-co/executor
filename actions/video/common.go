package video_common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// DefaultTimeout / MaxTimeout bound an ffmpeg invocation (mirrors the makefile action).
	DefaultTimeout = 120 * time.Second
	MaxTimeout     = 600 * time.Second
	// stderrCap limits how much ffmpeg stderr we retain for error reporting.
	stderrCap = 256 * 1024
)

// FFmpegPath resolves the ffmpeg binary: FLOMATION_FFMPEG_PATH override first,
// then PATH. No runtime download — the runner host must have ffmpeg installed.
func FFmpegPath() (string, error) { return resolveBinary("ffmpeg", "FLOMATION_FFMPEG_PATH") }

// FFprobePath resolves the ffprobe binary the same way.
func FFprobePath() (string, error) { return resolveBinary("ffprobe", "FLOMATION_FFPROBE_PATH") }

func resolveBinary(name, env string) (string, error) {
	if p := os.Getenv(env); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%s=%q does not exist", env, p)
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH — install it on the runner host", name)
	}
	return p, nil
}

// limitedBuffer is a bytes.Buffer that silently drops writes past a cap, so a
// runaway ffmpeg stderr can never blow up memory.
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
	return len(p), nil // report full length so ffmpeg never sees a short write
}

func (l *limitedBuffer) String() string { return l.buf.String() }

// RunFFmpeg executes ffmpeg with the given argv (NO shell — every arg is
// action-controlled), capturing capped stderr. The context carries the timeout.
func RunFFmpeg(ctx context.Context, args ...string) (string, error) {
	bin, err := FFmpegPath()
	if err != nil {
		return "", err
	}
	// #nosec G204 -- args are built from typed inputs, never a raw command string.
	cmd := exec.CommandContext(ctx, bin, args...)
	stderr := &limitedBuffer{cap: stderrCap}
	cmd.Stderr = stderr
	err = cmd.Run()
	return stderr.String(), err
}

// ProbeResult is the subset of ffprobe output the actions surface.
type ProbeResult struct {
	DurationSeconds float64
	Format          string
	VideoCodec      string
	AudioCodec      string
	Width           int
	Height          int
	BitRate         int
}

// Probe runs ffprobe on a file and returns its format/stream summary.
func Probe(ctx context.Context, path string) (*ProbeResult, error) {
	bin, err := FFprobePath()
	if err != nil {
		return nil, err
	}
	// #nosec G204 -- path is a workspace-confined file, not a shell string.
	cmd := exec.CommandContext(ctx, bin, "-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var raw struct {
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	res := &ProbeResult{Format: raw.Format.FormatName}
	res.DurationSeconds, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	res.BitRate, _ = strconv.Atoi(raw.Format.BitRate)
	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			if res.VideoCodec == "" {
				res.VideoCodec = s.CodecName
				res.Width, res.Height = s.Width, s.Height
			}
		case "audio":
			if res.AudioCodec == "" {
				res.AudioCodec = s.CodecName
			}
		}
	}
	return res, nil
}

// ── Input helpers ──

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

func OptionalBool(name string, def bool, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	switch v := c.Value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return def
}

func OptionalFloat(name string, def float64, inputs []*core.Connection) float64 {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	switch v := c.Value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return n
		}
	}
	return def
}

// ── Result helpers ──

// ErrResult builds the standard failure output map (success=false with the error
// surfaced in tool_result so an AI caller sees it). Returns a nil error so the
// map is delivered rather than swallowed.
func ErrResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"success":     false,
		"error":       msg,
	}, nil
}

// Tail returns the last n characters of s (for trimming long stderr into errors).
func Tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
