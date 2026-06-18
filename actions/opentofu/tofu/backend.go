package tofu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrLocalBackend is returned by RequireRemoteBackend when the working
// directory has no remote backend (or explicitly selects local state). Local
// state is unsafe in the stateless executor: it is lost between runs and a
// suspend/resume may move to a different runner that never sees the file.
var ErrLocalBackend = fmt.Errorf(
	"no remote backend configured: OpenTofu would use local state, which is unsafe in the stateless executor " +
		"(state is lost between runs and across suspend/resume). Configure a remote backend — e.g. a " +
		`backend "s3" / "gcs" / "azurerm" / "http" block, or a cloud {} block — or explicitly enable ` +
		"\"Allow Local State\" if you understand the risk")

var (
	reBackend      = regexp.MustCompile(`(?m)\bbackend\s+"([a-zA-Z0-9_-]+)"`)
	reCloud        = regexp.MustCompile(`(?m)\bcloud\s*\{`)
	reLineComment  = regexp.MustCompile(`(?m)(#|//).*$`)
	reBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// RequireRemoteBackend scans the .tf / .tf.json files in workDir and returns nil
// only when a non-local backend (or cloud block) is configured. It returns
// ErrLocalBackend when the configuration uses local state, and a descriptive
// error when no OpenTofu configuration is present at all.
//
// This is a guard rail, not a parser: it looks for a backend/cloud declaration
// rather than validating the whole configuration. Backends can only be declared
// in the root module, so a top-level scan is sufficient.
func RequireRemoteBackend(workDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return fmt.Errorf("reading working directory: %w", err)
	}

	sawConfig := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join(workDir, name)

		switch {
		case strings.HasSuffix(name, ".tf.json"):
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			sawConfig = true
			if t, ok := backendTypeJSON(data); ok && strings.ToLower(t) != "local" {
				return nil
			}
			if cloudJSON(data) {
				return nil
			}

		case strings.HasSuffix(name, ".tf"):
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			sawConfig = true
			src := stripComments(string(data))
			if reCloud.MatchString(src) {
				return nil
			}
			for _, m := range reBackend.FindAllStringSubmatch(src, -1) {
				if strings.ToLower(m[1]) != "local" {
					return nil
				}
			}
		}
	}

	if !sawConfig {
		return fmt.Errorf("no OpenTofu configuration (.tf / .tf.json) found in %q", workDir)
	}
	return ErrLocalBackend
}

// BackendAuthEnv translates a selected backend-auth provider and its credential
// fields into the environment variables understood by the matching OpenTofu
// backend. get returns the (already secret-resolved) value for an input name, or
// "" when unset; only non-empty values are emitted so partial selections don't
// inject blank credentials.
func BackendAuthEnv(provider string, get func(name string) string) map[string]string {
	env := map[string]string{}
	set := func(envVar, field string) {
		if v := strings.TrimSpace(get(field)); v != "" {
			env[envVar] = v
		}
	}

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws", "s3":
		set("AWS_ACCESS_KEY_ID", "aws_access_key_id")
		set("AWS_SECRET_ACCESS_KEY", "aws_secret_access_key")
		set("AWS_SESSION_TOKEN", "aws_session_token")
		set("AWS_REGION", "aws_region")
	case "azure", "azurerm":
		set("ARM_CLIENT_ID", "arm_client_id")
		set("ARM_CLIENT_SECRET", "arm_client_secret")
		set("ARM_TENANT_ID", "arm_tenant_id")
		set("ARM_SUBSCRIPTION_ID", "arm_subscription_id")
		set("ARM_ACCESS_KEY", "arm_access_key")
	case "gcp", "gcs", "google":
		// GOOGLE_CREDENTIALS accepts the service-account key JSON inline.
		set("GOOGLE_CREDENTIALS", "gcp_credentials_json")
	case "gitlab", "http":
		// GitLab-managed Terraform state uses the http backend; the state
		// address/lock URLs are supplied via backend_config, auth via env.
		set("TF_HTTP_USERNAME", "gitlab_username")
		set("TF_HTTP_PASSWORD", "gitlab_token")
	}
	return env
}

// BackendConfigFor derives additional `-backend-config` entries implied by the
// selected backend-auth provider. Currently this fills in the GitLab http
// backend's address/lock plumbing from a single state address, so the user only
// supplies one URL instead of six settings. get returns the (already resolved)
// value for an input name, or "". Returns an empty map when nothing applies;
// callers should let explicit backend_config entries override these.
func BackendConfigFor(provider string, get func(name string) string) map[string]string {
	cfg := map[string]string{}

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gitlab", "http":
		addr := strings.TrimSpace(get("gitlab_address"))
		if addr == "" {
			return cfg
		}
		// GitLab-managed state locks against <address>/lock with POST/DELETE,
		// per the GitLab http-backend documentation.
		lock := strings.TrimRight(addr, "/") + "/lock"
		cfg["address"] = addr
		cfg["lock_address"] = lock
		cfg["unlock_address"] = lock
		cfg["lock_method"] = "POST"
		cfg["unlock_method"] = "DELETE"
		cfg["retry_wait_min"] = "5"
	}
	return cfg
}

func stripComments(s string) string {
	s = reBlockComment.ReplaceAllString(s, "")
	s = reLineComment.ReplaceAllString(s, "")
	return s
}

func backendTypeJSON(data []byte) (string, bool) {
	for _, tf := range terraformBlocks(data) {
		if b, ok := tf["backend"]; ok {
			for _, bm := range asMaps(b) {
				for k := range bm {
					return k, true
				}
			}
		}
	}
	return "", false
}

func cloudJSON(data []byte) bool {
	for _, tf := range terraformBlocks(data) {
		if _, ok := tf["cloud"]; ok {
			return true
		}
	}
	return false
}

// terraformBlocks returns the terraform{} block(s) from a .tf.json document.
// The terraform key may be a single object or an array of objects.
func terraformBlocks(data []byte) []map[string]any {
	var root map[string]any
	if json.Unmarshal(data, &root) != nil {
		return nil
	}
	return asMaps(root["terraform"])
}

func asMaps(v any) []map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return []map[string]any{t}
	case []any:
		var out []map[string]any
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}
