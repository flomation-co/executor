package tofu

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTF(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRequireRemoteBackend(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		body    string
		wantErr error
	}{
		{
			name: "s3 backend ok",
			file: "main.tf",
			body: `terraform { backend "s3" { bucket = "b" } }`,
		},
		{
			name: "gcs backend ok",
			file: "main.tf",
			body: `terraform {
  backend "gcs" {}
}`,
		},
		{
			name: "cloud block ok",
			file: "main.tf",
			body: `terraform { cloud { organization = "acme" } }`,
		},
		{
			name: "json backend ok",
			file: "main.tf.json",
			body: `{"terraform": {"backend": {"azurerm": {}}}}`,
		},
		{
			name:    "explicit local rejected",
			file:    "main.tf",
			body:    `terraform { backend "local" {} }`,
			wantErr: ErrLocalBackend,
		},
		{
			name:    "no backend rejected",
			file:    "main.tf",
			body:    `resource "null_resource" "x" {}`,
			wantErr: ErrLocalBackend,
		},
		{
			name:    "commented-out backend rejected",
			file:    "main.tf",
			body:    "# terraform { backend \"s3\" {} }\nresource \"null_resource\" \"x\" {}",
			wantErr: ErrLocalBackend,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeTF(t, tc.file, tc.body)
			err := RequireRemoteBackend(dir)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRequireRemoteBackendNoConfig(t *testing.T) {
	dir := t.TempDir()
	if err := RequireRemoteBackend(dir); err == nil {
		t.Fatal("expected error for directory with no .tf files")
	}
}

func TestBackendAuthEnv(t *testing.T) {
	fields := map[string]string{
		"aws_access_key_id":     "AKIA",
		"aws_secret_access_key": "secret",
		"aws_region":            "eu-west-2",
		"arm_client_id":         "cid",
		"gcp_credentials_json":  `{"type":"service_account"}`,
		"gitlab_username":       "ci",
		"gitlab_token":          "glpat-xxx",
	}
	get := func(n string) string { return fields[n] }

	aws := BackendAuthEnv("aws", get)
	if aws["AWS_ACCESS_KEY_ID"] != "AKIA" || aws["AWS_SECRET_ACCESS_KEY"] != "secret" || aws["AWS_REGION"] != "eu-west-2" {
		t.Fatalf("unexpected aws env: %v", aws)
	}
	if _, ok := aws["AWS_SESSION_TOKEN"]; ok {
		t.Fatalf("empty session token should not be set: %v", aws)
	}

	if env := BackendAuthEnv("gcp", get); env["GOOGLE_CREDENTIALS"] != `{"type":"service_account"}` {
		t.Fatalf("unexpected gcp env: %v", env)
	}
	if env := BackendAuthEnv("gitlab", get); env["TF_HTTP_USERNAME"] != "ci" || env["TF_HTTP_PASSWORD"] != "glpat-xxx" {
		t.Fatalf("unexpected gitlab env: %v", env)
	}
	if env := BackendAuthEnv("", get); len(env) != 0 {
		t.Fatalf("expected no env for empty provider, got %v", env)
	}
}

func TestBackendConfigFor(t *testing.T) {
	get := func(n string) string {
		if n == "gitlab_address" {
			return "https://gitlab.com/api/v4/projects/42/terraform/state/prod"
		}
		return ""
	}

	cfg := BackendConfigFor("gitlab", get)
	wantLock := "https://gitlab.com/api/v4/projects/42/terraform/state/prod/lock"
	checks := map[string]string{
		"address":        "https://gitlab.com/api/v4/projects/42/terraform/state/prod",
		"lock_address":   wantLock,
		"unlock_address": wantLock,
		"lock_method":    "POST",
		"unlock_method":  "DELETE",
		"retry_wait_min": "5",
	}
	for k, want := range checks {
		if cfg[k] != want {
			t.Fatalf("gitlab backend config %q = %q, want %q", k, cfg[k], want)
		}
	}

	// No address → nothing derived.
	if cfg := BackendConfigFor("gitlab", func(string) string { return "" }); len(cfg) != 0 {
		t.Fatalf("expected empty config without address, got %v", cfg)
	}
	// Non-gitlab provider → nothing derived.
	if cfg := BackendConfigFor("aws", get); len(cfg) != 0 {
		t.Fatalf("expected empty config for aws, got %v", cfg)
	}
}
