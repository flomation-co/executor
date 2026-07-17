package devops_azuredevops_workitem_query_wiql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Query Work Items (WIQL)"
	Description  = "Run a WIQL query and return the matching work items, fully populated. WIQL itself returns ONLY id references no matter what the SELECT clause lists, so this action hydrates them for you (in batches of 200) — otherwise the results would be unusable in a flow. Cap your query with a WHERE clause: large result sets are expensive and are trimmed at 2000 items."
	Website      = "https://www.flomation.co"
	Icon         = "azure+magnifying-glass"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "query", Type: core.ConnectionTypeText, Label: "WIQL Query", Placeholder: "SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'Active' ORDER BY [System.ChangedDate] DESC", Required: true},
	{Name: "team", Type: core.ConnectionTypeString, Label: "Team", Placeholder: "team name — needed only for queries using @CurrentIteration or @TeamAreas"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "comma-separated reference names to return, e.g. System.Title,System.State — blank for the default set"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Work Items"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := azuredevops.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	project, err := azuredevops.RequiredString("project", "Project", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	query, err := azuredevops.RequiredString("query", "WIQL Query", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	// A team-scoped WIQL context is a longer path, not a parameter — it is what
	// makes @CurrentIteration and @TeamAreas resolve.
	path := azuredevops.ProjectPath(project) + "/_apis/wit/wiql"
	if team := azuredevops.OptionalString("team", inputs); team != "" {
		path = azuredevops.ProjectPath(project) + azuredevops.ProjectPath(team) + "/_apis/wit/wiql"
	}

	limit, set := azuredevops.OptionalInt("limit", inputs)
	want := azuredevops.ClampLimit(limit, set)

	q := url.Values{}
	q.Set("$top", strconv.Itoa(want))

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodPost,
		Path:   path,
		Query:  q,
		Body:   map[string]interface{}{"query": query},
	})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	var refs struct {
		WorkItems []struct {
			ID int `json:"id"`
		} `json:"workItems"`
		WorkItemRelations []struct {
			Target struct {
				ID int `json:"id"`
			} `json:"target"`
		} `json:"workItemRelations"`
	}
	if err := json.Unmarshal(resp.Body, &refs); err != nil {
		return azuredevops.ErrorResult("failed to parse the WIQL response: " + err.Error()), nil
	}

	ids := []int{}
	seen := map[int]bool{}
	add := func(id int) {
		if id != 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, r := range refs.WorkItems {
		add(r.ID)
	}
	// A tree/one-hop WIQL query answers with workItemRelations instead of
	// workItems, and the same item can appear as several relations' target —
	// hence the dedupe. Without this branch those query shapes return nothing.
	for _, r := range refs.WorkItemRelations {
		add(r.Target.ID)
	}

	if len(ids) == 0 {
		return azuredevops.ListResult(nil, "The query matched no work items"), nil
	}
	trimmed := false
	if len(ids) > want {
		ids, trimmed = ids[:want], true
	}
	if len(ids) > azuredevops.MaxWiqlResults {
		ids, trimmed = ids[:azuredevops.MaxWiqlResults], true
	}

	fields := azuredevops.SplitCommaList(azuredevops.OptionalString("fields", inputs))
	items, err := azuredevops.FetchWorkItems(flow, auth, project, ids, fields, "")
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d work item(s)", len(items))
	if trimmed {
		summary += " — more matched; narrow the query or raise Limit"
	}
	return azuredevops.ListResult(items, summary), nil
}
