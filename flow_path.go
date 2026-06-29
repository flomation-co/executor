// Reference-path resolution for variable substitution.
//
// Previously ${namespace.root} resolved to a single top-level value
// per namespace+root pair. This file extends that to ${namespace.root.x.y[0].z}
// so flow authors can drill into structured outputs (Web/HTTP JSON
// bodies, AI responses, SQL row maps, etc.) without an intermediate
// extraction action.
//
// Two surfaces:
//
//   1. ParseReference splits a ${...} body like "node_abc.body.items[3].id"
//      into root="node_abc", child="body", segments=["items","3","id"].
//      For namespace-prefixed references like "flow.user.profile.email",
//      root="flow", child="user", segments=["profile","email"].
//
//   2. ResolvePath walks an interface{} value via the parsed segments.
//      Map fields are indexed by string segment; numeric segments index
//      slices. If the input value is a string at the entry point, an
//      attempt is made to JSON-parse it before traversing — this lets
//      ${web_request.response_body.user.id} work even though
//      response_body is a raw JSON string (the Web action's output
//      type is ConnectionTypeString).
//
// Both helpers are pure functions, fully unit-tested, and have no
// dependency on the Flow type. They're imported by substituteVariables
// in flow.go.

package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// PathReference holds the result of ParseReference: a 3-tuple of
// namespace/root + child key + remaining path segments. The shape
// matches how the existing substitution code is organised — each
// namespace branch (flow/var/user/secrets/etc.) gets the child key
// to drive its first-level lookup, then walks the segments if any.
//
// For unprefixed references like "node_abc.field", Namespace is empty,
// Root is "node_abc", Child is "field", Path is empty.
type PathReference struct {
	// Namespace, when set, is one of the recognised prefixes
	// ("flow", "var", "user", "secrets", "credentials", etc.).
	// Empty namespace means "this is a parent-node-output reference".
	Namespace string
	// Root is the first segment after the namespace — for namespace
	// references it's the variable name; for parent-output
	// references it's the node ID.
	Root string
	// Child is the SECOND segment — for namespace references it's
	// typically the variable's first-level field; for parent-output
	// references it's the output key on the parent node.
	//
	// Note this is the key existing code already passes to its
	// namespace lookups (e.g. flow.X has child="X"). Path support
	// preserves that contract — child is what the existing lookup
	// returns; Path is what we then walk INTO that result.
	Child string
	// Path is everything after the namespace+root+child. Empty when
	// the reference has no further depth (backward-compat shape).
	Path []string
}

// ParseReference tokenises a ${...} body into the PathReference shape.
// Recognises dot separators and bracket indices in any combination
// after the namespace+root pair:
//
//	"flow.x"                         -> {ns:"flow", root:"x"}
//	"flow.x.y"                       -> {ns:"flow", root:"x", child:"y"}
//	"flow.x.y.z"                     -> {ns:"flow", root:"x", child:"y", path:["z"]}
//	"flow.x.items[0].id"             -> {ns:"flow", root:"x", child:"items", path:["0","id"]}
//	"node_abc.body"                  -> {ns:"", root:"node_abc", child:"body"}
//	"node_abc.body.user.name"        -> {ns:"", root:"node_abc", child:"body", path:["user","name"]}
//	"node_abc.body[2]"               -> {ns:"", root:"node_abc", child:"body", path:["2"]}
//
// The namespace detection is hardcoded to the set the executor
// recognises — anything not in that set is treated as a parent-node
// output reference. Order matters in the prefix set: longer aliases
// (credentials vs credential) come first so we don't mis-match.
//
// Returns ok=false for malformed references (no separators, only
// whitespace, etc.) so callers can fall back to the previous
// behaviour (replace with empty string + log).
func ParseReference(ref string) (PathReference, bool) {
	if ref == "" {
		return PathReference{}, false
	}

	// First tokenise the whole thing into segments. Dots split
	// segments; bracketed indices are their own segments. So
	// "x.items[3].id" → ["x", "items", "3", "id"].
	segments := tokenisePath(ref)
	if len(segments) < 2 {
		// Need at least namespace+root OR root+child for any
		// substitution to make sense. Bare "${node_abc}" with
		// nothing after is not a meaningful reference.
		return PathReference{}, false
	}

	out := PathReference{}

	knownNamespaces := []string{
		"secrets", "secret",
		"credentials", "credential",
		"env",
		"flow",
		"var",
		"user",
		"loop",
		"trigger",
		"input",
	}
	for _, ns := range knownNamespaces {
		if segments[0] == ns {
			out.Namespace = ns
			break
		}
	}

	if out.Namespace != "" {
		// "ns.root[.child[.path...]]"
		out.Root = segments[1]
		if len(segments) >= 3 {
			out.Child = segments[2]
			if len(segments) >= 4 {
				out.Path = segments[3:]
			}
		}
	} else {
		// "root.child[.path...]"  (parent-node-output reference)
		out.Root = segments[0]
		out.Child = segments[1]
		if len(segments) >= 3 {
			out.Path = segments[2:]
		}
	}
	return out, true
}

// tokenisePath breaks a reference string into atomic segments,
// honouring both dot and bracket notation. Empty segments (from
// leading/trailing dots or stray brackets) are silently dropped.
//
// "x.y[0].z" → ["x","y","0","z"]
// "items[3]"  → ["items","3"]
// "x.0.y"     → ["x","0","y"]   (numeric dot segments stay numeric;
//
//	we don't distinguish — the resolver handles both forms identically)
func tokenisePath(ref string) []string {
	var segments []string
	var current strings.Builder
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		switch c {
		case '.':
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		case '[':
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		case ']':
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	return segments
}

// ResolvePath descends into v via the supplied path segments.
//
//   - Map[string]interface{} segments resolve by exact key match.
//   - Slice (and []interface{}, []map[string]interface{}) segments
//     resolve by integer index parsed from the segment string.
//   - When v is a string at the entry point, an attempt is made to
//     JSON-parse it into a generic value before traversing. This is
//     the load-bearing affordance for Web/HTTP outputs whose
//     response_body is a raw JSON string. Path lookup against a
//     non-JSON string returns an error.
//   - When the path is empty, the input is returned unchanged.
//   - When a segment doesn't resolve (missing map key, out-of-range
//     index, descent into a scalar), an error is returned naming the
//     failing segment so callers can surface clear diagnostics.
//
// Returns the resolved value plus nil error on success.
func ResolvePath(v interface{}, path []string) (interface{}, error) {
	if len(path) == 0 {
		return v, nil
	}

	// Promote a string root to a parsed JSON value when it looks
	// like one. The Web/HTTP common case: response_body is a
	// JSON-shaped string in the executor's typed output. We
	// only ATTEMPT the parse — failure leaves v as the string
	// and the next-step traversal surfaces a clear error.
	if s, ok := v.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			v = parsed
		}
	}

	current := v
	for i, seg := range path {
		if seg == "" {
			continue
		}
		switch typed := current.(type) {
		case map[string]interface{}:
			next, ok := typed[seg]
			if !ok {
				return nil, fmt.Errorf("path segment %q not found at %q", seg, strings.Join(path[:i+1], "."))
			}
			current = next
		case []interface{}:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return nil, fmt.Errorf("segment %q is not a numeric index at %q", seg, strings.Join(path[:i+1], "."))
			}
			if idx < 0 || idx >= len(typed) {
				return nil, fmt.Errorf("index %d out of bounds (length %d) at %q", idx, len(typed), strings.Join(path[:i+1], "."))
			}
			current = typed[idx]
		case []map[string]interface{}:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return nil, fmt.Errorf("segment %q is not a numeric index at %q", seg, strings.Join(path[:i+1], "."))
			}
			if idx < 0 || idx >= len(typed) {
				return nil, fmt.Errorf("index %d out of bounds (length %d) at %q", idx, len(typed), strings.Join(path[:i+1], "."))
			}
			current = typed[idx]
		default:
			return nil, fmt.Errorf("can't descend into %T at segment %q (path %q)", current, seg, strings.Join(path[:i+1], "."))
		}
	}
	return current, nil
}
