package gitlab_common

import (
	"testing"
)

func TestEncodeProjectID_Numeric(t *testing.T) {
	if got := EncodeProjectID("42"); got != "42" {
		t.Errorf("EncodeProjectID(\"42\") = %q, want \"42\"", got)
	}
}

func TestEncodeProjectID_Namespaced(t *testing.T) {
	got := EncodeProjectID("flomation/api")
	want := "flomation%2Fapi"
	if got != want {
		t.Errorf("EncodeProjectID(\"flomation/api\") = %q, want %q", got, want)
	}
}

func TestEncodeProjectID_DeepNamespace(t *testing.T) {
	got := EncodeProjectID("org/group/project")
	want := "org%2Fgroup%2Fproject"
	if got != want {
		t.Errorf("EncodeProjectID(\"org/group/project\") = %q, want %q", got, want)
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		baseURL string
		path    string
		want    string
	}{
		{"https://gitlab.com", "/projects/42/merge_requests", "https://gitlab.com/api/v4/projects/42/merge_requests"},
		{"https://gitlab.com/", "/projects/42", "https://gitlab.com/api/v4/projects/42"},
		{"https://gitlab.example.com", "/projects/42", "https://gitlab.example.com/api/v4/projects/42"},
	}

	for _, tt := range tests {
		got := BuildURL(tt.baseURL, tt.path)
		if got != tt.want {
			t.Errorf("BuildURL(%q, %q) = %q, want %q", tt.baseURL, tt.path, got, tt.want)
		}
	}
}

func TestBuildProjectURL(t *testing.T) {
	got := BuildProjectURL("https://gitlab.com", "flomation/api", "/merge_requests")
	want := "https://gitlab.com/api/v4/projects/flomation%2Fapi/merge_requests"
	if got != want {
		t.Errorf("BuildProjectURL() = %q, want %q", got, want)
	}
}

func TestBuildProjectURL_Numeric(t *testing.T) {
	got := BuildProjectURL("https://gitlab.com", "8", "/pipelines")
	want := "https://gitlab.com/api/v4/projects/8/pipelines"
	if got != want {
		t.Errorf("BuildProjectURL() = %q, want %q", got, want)
	}
}

func TestGetBaseURL_Default(t *testing.T) {
	got := GetBaseURL(nil)
	if got != DefaultBaseURL {
		t.Errorf("GetBaseURL(nil) = %q, want %q", got, DefaultBaseURL)
	}
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("something failed")
	if r["tool_result"] != "something failed" {
		t.Errorf("tool_result = %v, want \"something failed\"", r["tool_result"])
	}
	if r["success"] != false {
		t.Errorf("success = %v, want false", r["success"])
	}
	if r["error"] != "something failed" {
		t.Errorf("error = %v, want \"something failed\"", r["error"])
	}
}
