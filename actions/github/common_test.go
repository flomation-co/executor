package github_common

import (
	"testing"
)

func TestBuildURL_Default(t *testing.T) {
	got := BuildURL("https://api.github.com", "/repos/owner/repo/pulls")
	want := "https://api.github.com/repos/owner/repo/pulls"
	if got != want {
		t.Errorf("BuildURL() = %q, want %q", got, want)
	}
}

func TestBuildURL_Enterprise(t *testing.T) {
	got := BuildURL("https://github.example.com", "/repos/owner/repo/pulls")
	want := "https://github.example.com/api/v3/repos/owner/repo/pulls"
	if got != want {
		t.Errorf("BuildURL() = %q, want %q", got, want)
	}
}

func TestBuildURL_EnterpriseWithSuffix(t *testing.T) {
	got := BuildURL("https://github.example.com/api/v3", "/repos/owner/repo")
	want := "https://github.example.com/api/v3/repos/owner/repo"
	if got != want {
		t.Errorf("BuildURL() = %q, want %q", got, want)
	}
}

func TestBuildURL_TrailingSlash(t *testing.T) {
	got := BuildURL("https://api.github.com/", "/repos/owner/repo")
	want := "https://api.github.com/repos/owner/repo"
	if got != want {
		t.Errorf("BuildURL() = %q, want %q", got, want)
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
}
