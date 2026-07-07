package asana_task_remove_tag

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Remove Tag from Task"
	Description  = "Remove a tag from an Asana task."
	Website      = "https://www.flomation.co"
	Icon         = "asana+xmark"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "The task to untag", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "Optional — used only to load the Tag picker (not sent to Asana)"},
	{Name: "tag", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "The tag to remove", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := asana.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	taskID, err := asana.RequiredString("task_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	tag, err := asana.RequiredString("tag", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"tag": tag}
	if _, err := asana.WriteObject(auth, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/removeTag", body, url.Values{}); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.SuccessResult(taskID, map[string]interface{}{"task": taskID, "tag": tag}, "Removed tag "+tag+" from task "+taskID), nil
}
