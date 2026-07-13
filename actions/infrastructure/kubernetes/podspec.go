// Pod-spec construction helpers shared by the actions that build a container
// from scratch — cronjob_create and job_create both turn a human-entered image,
// command line, and environment map into the nested JSON a PodSpec needs.
//
// They live here, rather than being duplicated in each action, because the two
// actions must agree byte-for-byte on how a command string is tokenised: a Job
// created directly and a Job spawned from a CronJob template should run the same
// argv for the same input. Keeping the splitter in one place is what guarantees
// that.
package kubernetes

import (
	"fmt"
	"sort"
	"strings"
)

// SplitCommand parses a shell-style command line into the argv slice a
// container's `command` (or `args`) field takes.
//
// It honours the three quoting mechanisms a person actually types — single
// quotes (everything literal), double quotes (literal but backslash still
// escapes " \ $ `), and a bare backslash escape — and splits on unquoted
// whitespace. It deliberately does NOT expand variables, globs, or shell
// operators: the string becomes a container's argv, which the kernel execs
// directly, so `$HOME` or `*` are passed through as literal characters exactly
// as they would be to execve, not interpreted.
//
// An unbalanced quote or a dangling trailing backslash is an error, not a
// silently-truncated argument: a mis-typed command should be rejected at build
// time with a message, not fail obscurely inside the container.
//
// An empty or all-whitespace string yields a nil slice, which callers treat as
// "no command override" (the image's own ENTRYPOINT stands).
func SplitCommand(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg := false

	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n; i++ {
		c := runes[i]
		switch c {
		case '\'':
			// Single quotes: everything up to the next single quote is literal.
			inArg = true
			i++
			for i < n && runes[i] != '\'' {
				cur.WriteRune(runes[i])
				i++
			}
			if i >= n {
				return nil, fmt.Errorf("unbalanced single quote in command")
			}
		case '"':
			// Double quotes: literal, except backslash still escapes a handful of
			// characters (matching POSIX sh) so `"\$x"` yields a literal `$x`.
			inArg = true
			i++
			for i < n && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < n {
					switch runes[i+1] {
					case '"', '\\', '$', '`':
						cur.WriteRune(runes[i+1])
						i += 2
						continue
					}
				}
				cur.WriteRune(runes[i])
				i++
			}
			if i >= n {
				return nil, fmt.Errorf("unbalanced double quote in command")
			}
		case '\\':
			// A bare backslash escapes the next character literally.
			if i+1 >= n {
				return nil, fmt.Errorf("command ends with a dangling backslash")
			}
			cur.WriteRune(runes[i+1])
			inArg = true
			i++
		case ' ', '\t', '\n', '\r':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(c)
			inArg = true
		}
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}

// EnvList converts a flat name→value map into the ordered list of {name,value}
// objects a container's `env` field takes.
//
// Keys are sorted so the same map always renders the same list. Go map
// iteration is randomised, and a container spec whose env order flips between
// runs would make an otherwise-unchanged object churn — a needless new revision
// on every apply. The return type is []any because it drops straight into the
// map[string]interface{} object bodies these actions hand to json.Marshal.
func EnvList(env map[string]string) []any {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]interface{}{"name": k, "value": env[k]})
	}
	return out
}
