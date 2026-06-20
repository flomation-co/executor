package core

// Tokenisation policy for large tool outputs and rehydration of blob
// tokens in incoming tool arguments. Pure functions over maps —
// statelessness keeps the AI-loop site simple to reason about.

import (
	"errors"
	"strings"
)

// mediaKeySuffixes identifies fields whose values are almost
// certainly bulk media data. Combined with the size threshold, this
// is the "obviously off-loadable" set — we tokenise these without
// further sniffing because their names already declare intent.
var mediaKeySuffixes = []string{
	"_base64",
	"_data",
	"_content",
	"_audio",
	"_image",
	"_video",
	"_bytes",
	"_blob",
}

// mediaMimeByKey is a coarse lookup of canonical content types per
// key suffix — purely a hint to the LLM, never used for parsing.
// When a more specific MIME is available (e.g. audio_format on a
// TTS result) the caller can override.
var mediaMimeByKey = map[string]string{
	"_base64":  "application/octet-stream",
	"_data":    "application/octet-stream",
	"_content": "application/octet-stream",
	"_audio":   "audio/mpeg",
	"_image":   "image/png",
	"_video":   "video/mp4",
	"_bytes":   "application/octet-stream",
	"_blob":    "application/octet-stream",
}

// TokenManifestEntry pairs an output field name with the token that
// will appear in the LLM's view of the tool result. The AI loop
// uses these to format a manifest section appended to the
// tool_result string so the model can refer to them by name.
type TokenManifestEntry struct {
	Field string
	Token string
	Size  int
	Mime  string
}

// TokeniseLargeOutputs scans outputs and off-loads any string values
// that exceed BlobThresholdBytes AND whose key suggests bulk media.
// Returns the manifest of (field → token) pairs that the caller
// should advertise to the LLM. The outputs map is NOT mutated —
// the BlobStore retains the full value for graph-wired downstream
// nodes and for the editor's inspector/Download path.
//
// Specifically: we never tokenise the outputs map that is stored
// in node_results. Only the AI-visible projection (the
// tool_result string) carries tokens. This is the load-bearing
// simplification of the whole design — DB stays full-fidelity,
// auto-wiring keeps working, editor display unchanged.
func TokeniseLargeOutputs(outputs map[string]interface{}, store *BlobStore) []TokenManifestEntry {
	if store == nil || len(outputs) == 0 {
		return nil
	}
	var manifest []TokenManifestEntry
	// Prefer the format hint from a paired audio_format/output_format
	// field if present, so the LLM sees audio/ogg rather than the
	// generic audio/mpeg when the action actually produced OGG.
	mimeOverride := mimeOverrideFor(outputs)

	for key, raw := range outputs {
		s, ok := raw.(string)
		if !ok {
			continue
		}
		if len(s) < BlobThresholdBytes {
			continue
		}
		if !looksMediaShaped(key) {
			continue
		}
		mime := mimeOverride
		if mime == "" {
			mime = mimeForKey(key)
		}
		token, err := store.Put([]byte(s), mime)
		if err != nil {
			// A store failure is non-fatal: the LLM falls back to
			// seeing nothing for this field rather than the full
			// blob. The action still has the real value in its
			// outputs map, so downstream graph-wired nodes are
			// unaffected.
			continue
		}
		manifest = append(manifest, TokenManifestEntry{
			Field: key,
			Token: token,
			Size:  len(s),
			Mime:  mime,
		})
	}
	return manifest
}

// DetokeniseInputs walks the LLM-supplied tool argument map and
// replaces any value that is a verbatim blob token with the
// resolved bytes. Non-token strings pass through unchanged.
//
// Errors carry the offending field name so callers can build a
// helpful tool-loop error message:
//
//	"audio_base64 referenced an unknown blob — pass the token
//	 verbatim from the previous tool result"
//
// Returns the new args map (always a fresh map, never the input)
// plus any error from the FIRST failed resolution. Subsequent
// errors are silently ignored — one clear message beats a wall.
func DetokeniseInputs(args map[string]interface{}, store *BlobStore) (map[string]interface{}, error) {
	if len(args) == 0 {
		return args, nil
	}
	out := make(map[string]interface{}, len(args))
	var firstErr error
	for k, v := range args {
		s, ok := v.(string)
		if !ok || !IsBlobToken(s) {
			out[k] = v
			continue
		}
		if store == nil {
			if firstErr == nil {
				firstErr = errors.New(k + ": blob token received but no blob store available")
			}
			out[k] = v
			continue
		}
		data, err := store.Get(s)
		if err != nil {
			if firstErr == nil {
				firstErr = errors.New(k + ": " + err.Error())
			}
			out[k] = v
			continue
		}
		// Tools that expect binary data (audio_base64,
		// image_base64) expect the *string* form they were
		// originally given. We stored the raw bytes; convert back
		// to the original string representation. For non-string
		// payloads (binary uploads, etc.) callers that need raw
		// bytes can interrogate the store directly.
		out[k] = string(data)
	}
	return out, firstErr
}

// FormatTokenManifest produces the trailer appended to the
// LLM-visible tool_result string. Shape:
//
//	Outputs available as references (pass verbatim to downstream tools):
//	  audio_base64: flo:blob:a3f9c2d1b4e7805f?size=553436&type=audio%2Fmpeg
//
// Empty manifest returns empty string so callers can unconditionally
// append.
func FormatTokenManifest(manifest []TokenManifestEntry) string {
	if len(manifest) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\nOutputs available as references (pass verbatim to downstream tools):\n")
	for _, e := range manifest {
		sb.WriteString("  ")
		sb.WriteString(e.Field)
		sb.WriteString(": ")
		sb.WriteString(e.Token)
		sb.WriteString("\n")
	}
	return sb.String()
}

// looksMediaShaped applies the heuristic for which keys are
// candidates for off-loading. A pure suffix check is enough for
// today's action set; if we ever add an off-loadable field whose
// name doesn't match, we can either rename it or add a suffix.
func looksMediaShaped(key string) bool {
	lk := strings.ToLower(key)
	for _, suf := range mediaKeySuffixes {
		if strings.HasSuffix(lk, suf) {
			return true
		}
	}
	return false
}

func mimeForKey(key string) string {
	lk := strings.ToLower(key)
	for _, suf := range mediaKeySuffixes {
		if strings.HasSuffix(lk, suf) {
			return mediaMimeByKey[suf]
		}
	}
	return ""
}

// mimeOverrideFor looks at companion fields that often carry a more
// specific content type than the suffix-based default. Today only
// `audio_format` / `output_format` are recognised; trivial to
// extend as new patterns appear.
func mimeOverrideFor(outputs map[string]interface{}) string {
	for _, k := range []string{"audio_format", "output_format"} {
		if v, ok := outputs[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return canonicalFormatToMIME(s)
			}
		}
	}
	return ""
}

// canonicalFormatToMIME maps ElevenLabs-style format identifiers to
// their HTTP content type. Empty string for an unrecognised value
// means the caller falls through to the suffix default.
func canonicalFormatToMIME(format string) string {
	lf := strings.ToLower(format)
	switch {
	case strings.HasPrefix(lf, "mp3"):
		return "audio/mpeg"
	case strings.HasPrefix(lf, "pcm"):
		return "audio/L16"
	case strings.HasPrefix(lf, "ogg"), strings.HasPrefix(lf, "opus"):
		return "audio/ogg"
	case strings.HasPrefix(lf, "ulaw"):
		return "audio/basic"
	case strings.HasPrefix(lf, "wav"):
		return "audio/wav"
	}
	return ""
}
