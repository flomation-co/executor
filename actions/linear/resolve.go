package linear_common

import (
	"encoding/json"
	"fmt"
	"strings"
)

// LooksLikeUUID reports whether s is already a Linear UUID (so it needs no
// name resolution). Linear ids are 36-char dash-delimited UUIDs.
func LooksLikeUUID(s string) bool {
	return len(s) == 36 && strings.Count(s, "-") == 4
}

// ResolveLabelIDs maps a list of label NAMES (or UUIDs) to label UUIDs,
// case-insensitively. A value that already looks like a UUID passes through
// unchanged. Returns the resolved ids and any names it could not match, so the
// caller can surface a clear error instead of silently dropping labels.
//
// This is what breaks the "I need the label UUID and can't get it" loop: an
// agent passes "Enriched" and gets the id without ever handling a UUID itself.
func ResolveLabelIDs(apiKey string, values []string) (ids []string, unresolved []string, err error) {
	var needNames bool
	for _, v := range values {
		if !LooksLikeUUID(strings.TrimSpace(v)) {
			needNames = true
			break
		}
	}

	byName := map[string]string{}
	if needNames {
		resp, e := ExecuteGraphQL(apiKey, GraphQLRequest{
			Query: `query { issueLabels(first: 250) { nodes { id name } } }`,
		})
		if e != nil {
			return nil, nil, e
		}
		var out struct {
			IssueLabels struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"issueLabels"`
		}
		if e := json.Unmarshal(resp.Data, &out); e != nil {
			return nil, nil, e
		}
		for _, n := range out.IssueLabels.Nodes {
			byName[strings.ToLower(n.Name)] = n.ID
		}
	}

	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if LooksLikeUUID(v) {
			ids = append(ids, v)
			continue
		}
		if id, ok := byName[strings.ToLower(v)]; ok {
			ids = append(ids, id)
		} else {
			unresolved = append(unresolved, v)
		}
	}
	return ids, unresolved, nil
}

// ResolveUserID maps a user NAME, display name or email to a UUID. A value that
// already looks like a UUID passes through. Email match wins over name match.
func ResolveUserID(apiKey, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("no user provided")
	}
	if LooksLikeUUID(value) {
		return value, nil
	}

	resp, err := ExecuteGraphQL(apiKey, GraphQLRequest{
		Query: `query { users(first: 250) { nodes { id name displayName email } } }`,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Users struct {
			Nodes []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
				Email       string `json:"email"`
			} `json:"nodes"`
		} `json:"users"`
	}
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return "", err
	}
	lv := strings.ToLower(value)
	// Email is the strongest signal — check it first.
	for _, u := range out.Users.Nodes {
		if u.Email != "" && strings.EqualFold(u.Email, value) {
			return u.ID, nil
		}
	}
	for _, u := range out.Users.Nodes {
		if strings.ToLower(u.Name) == lv || strings.ToLower(u.DisplayName) == lv {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("no Linear user matched %q", value)
}

// ResolveProjectID maps a project NAME to a UUID. A UUID passes through.
func ResolveProjectID(apiKey, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("no project provided")
	}
	if LooksLikeUUID(value) {
		return value, nil
	}
	resp, err := ExecuteGraphQL(apiKey, GraphQLRequest{
		Query: `query { projects(first: 250) { nodes { id name } } }`,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Projects struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return "", err
	}
	for _, p := range out.Projects.Nodes {
		if strings.EqualFold(p.Name, value) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("no Linear project matched %q", value)
}

// SplitCSV splits a comma-separated input into trimmed, non-empty values.
func SplitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
