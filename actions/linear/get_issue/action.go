package linear_get_issue

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Issue"
	Description  = "Fetch a Linear issue by UUID or identifier (e.g. ENG-123). Returns full details."
	Website      = "https://www.flomation.co"
	Icon         = "linear"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "Linear API Key",
		Placeholder: "lin_api_...",
		Required:    true,
	},
	{
		Name:        "issue_id",
		Type:        core.ConnectionTypeString,
		Label:       "Issue ID or Identifier",
		Placeholder: "UUID or ENG-123",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue ID"},
	{Name: "identifier", Type: core.ConnectionTypeString, Label: "Identifier"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee"},
	{Name: "team", Type: core.ConnectionTypeString, Label: "Team"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL"},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "Due Date"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "updated_at", Type: core.ConnectionTypeString, Label: "Updated At"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Full Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

const issueByIDQuery = `query GetIssue($id: String!) {
	issue(id: $id) {
		id identifier title description url priority priorityLabel
		dueDate estimate createdAt updatedAt
		state { id name }
		assignee { id name email }
		team { id name key }
		project { id name }
		cycle { id name number }
		parent { id identifier title }
		children { nodes { id identifier title state { name } } }
		labels { nodes { id name color } }
		relations { nodes { type relatedIssue { id identifier title } } }
	}
}`

const issueByIdentifierQuery = `query GetIssueByIdentifier($filter: IssueFilter!) {
	issues(filter: $filter, first: 1) {
		nodes {
			id identifier title description url priority priorityLabel
			dueDate estimate createdAt updatedAt
			state { id name }
			assignee { id name email }
			team { id name key }
			project { id name }
			cycle { id name number }
			parent { id identifier title }
			children { nodes { id identifier title state { name } } }
			labels { nodes { id name color } }
			relations { nodes { type relatedIssue { id identifier title } } }
		}
	}
}`

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	issueID, err := linear.RequiredString("issue_id", inputs)
	if err != nil {
		return nil, err
	}

	// Detect identifier format (e.g. ENG-123) vs UUID.
	isIdentifier := len(issueID) < 36 && strings.Contains(issueID, "-") &&
		!strings.ContainsAny(issueID[:1], "0123456789")

	var resp *linear.GraphQLResponse
	if isIdentifier {
		resp, err = linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
			Query: issueByIdentifierQuery,
			Variables: map[string]interface{}{
				"filter": map[string]interface{}{
					"identifier": map[string]string{"eq": issueID},
				},
			},
		})
	} else {
		resp, err = linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
			Query:     issueByIDQuery,
			Variables: map[string]interface{}{"id": issueID},
		})
	}
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	type issueData struct {
		ID            string  `json:"id"`
		Identifier    string  `json:"identifier"`
		Title         string  `json:"title"`
		Description   string  `json:"description"`
		URL           string  `json:"url"`
		Priority      int     `json:"priority"`
		PriorityLabel string  `json:"priorityLabel"`
		DueDate       *string `json:"dueDate"`
		Estimate      *int    `json:"estimate"`
		CreatedAt     string  `json:"createdAt"`
		UpdatedAt     string  `json:"updatedAt"`
		State         *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"state"`
		Assignee *struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"assignee"`
		Team *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"team"`
		Project *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Cycle *struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Number int    `json:"number"`
		} `json:"cycle"`
		Parent *struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
			Title      string `json:"title"`
		} `json:"parent"`
		Children struct {
			Nodes []struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				Title      string `json:"title"`
				State      *struct {
					Name string `json:"name"`
				} `json:"state"`
			} `json:"nodes"`
		} `json:"children"`
		Labels struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"labels"`
		Relations struct {
			Nodes []struct {
				Type         string `json:"type"`
				RelatedIssue struct {
					ID         string `json:"id"`
					Identifier string `json:"identifier"`
					Title      string `json:"title"`
				} `json:"relatedIssue"`
			} `json:"nodes"`
		} `json:"relations"`
	}

	var issue issueData
	if isIdentifier {
		var result struct {
			Issues struct {
				Nodes []issueData `json:"nodes"`
			} `json:"issues"`
		}
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		if len(result.Issues.Nodes) == 0 {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Issue %s not found", issueID),
				"success":     false,
				"error":       "not found",
			}, nil
		}
		issue = result.Issues.Nodes[0]
	} else {
		var result struct {
			Issue issueData `json:"issue"`
		}
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		issue = result.Issue
	}
	stateName := ""
	if issue.State != nil {
		stateName = issue.State.Name
	}
	assigneeName := ""
	if issue.Assignee != nil {
		assigneeName = issue.Assignee.Name
	}
	teamName := ""
	if issue.Team != nil {
		teamName = issue.Team.Name
	}
	dueDate := ""
	if issue.DueDate != nil {
		dueDate = *issue.DueDate
	}

	var labelNames []string
	for _, l := range issue.Labels.Nodes {
		labelNames = append(labelNames, l.Name)
	}
	labelsStr := ""
	for i, n := range labelNames {
		if i > 0 {
			labelsStr += ", "
		}
		labelsStr += n
	}

	// Build detailed summary.
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s\n", issue.Identifier, issue.Title)
	fmt.Fprintf(&sb, "  State: %s | Priority: %s | Assignee: %s\n", stateName, issue.PriorityLabel, assigneeName)
	if teamName != "" {
		fmt.Fprintf(&sb, "  Team: %s\n", teamName)
	}
	if dueDate != "" {
		fmt.Fprintf(&sb, "  Due: %s\n", dueDate)
	}
	if labelsStr != "" {
		fmt.Fprintf(&sb, "  Labels: %s\n", labelsStr)
	}
	if issue.Parent != nil {
		fmt.Fprintf(&sb, "  Parent: %s — %s\n", issue.Parent.Identifier, issue.Parent.Title)
	}
	if len(issue.Children.Nodes) > 0 {
		fmt.Fprintf(&sb, "  Sub-issues (%d):\n", len(issue.Children.Nodes))
		for _, ch := range issue.Children.Nodes {
			chState := ""
			if ch.State != nil {
				chState = ch.State.Name
			}
			fmt.Fprintf(&sb, "    • %s: %s [%s]\n", ch.Identifier, ch.Title, chState)
		}
	}
	if issue.Description != "" {
		desc := issue.Description
		if len(desc) > 300 {
			desc = desc[:300] + "..."
		}
		fmt.Fprintf(&sb, "  Description: %s\n", desc)
	}

	// Build full result as JSON object.
	fullResult, _ := json.Marshal(issue)
	var fullObj interface{}
	_ = json.Unmarshal(fullResult, &fullObj)

	return map[string]interface{}{
		"tool_result": sb.String(),
		"issue_id":    issue.ID,
		"identifier":  issue.Identifier,
		"title":       issue.Title,
		"description": issue.Description,
		"state":       stateName,
		"priority":    issue.PriorityLabel,
		"assignee":    assigneeName,
		"team":        teamName,
		"url":         issue.URL,
		"due_date":    dueDate,
		"created_at":  issue.CreatedAt,
		"updated_at":  issue.UpdatedAt,
		"labels":      labelsStr,
		"result":      fullObj,
		"success":     true,
		"error":       "",
	}, nil
}
