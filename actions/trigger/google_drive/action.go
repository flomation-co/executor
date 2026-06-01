// Package google_drive triggers a flow when files are created, modified,
// or deleted in a Google Drive folder.
package google_drive

import (
	core "flomation.app/automate/executor"

	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Google Drive Trigger"
	Description  = "Triggers when files change in a Google Drive folder"
	Website      = "https://www.flomation.co"
	Icon         = "google"
	Date         = "01/06/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{
		Name:        "folder_id",
		Type:        core.ConnectionTypeString,
		Label:       "Folder ID",
		Placeholder: "root",
	},
	{
		Name:        "google_account",
		Type:        core.ConnectionTypeString,
		Label:       "Google Account (email)",
		Placeholder: "user@gmail.com",
	},
	{
		Name:        "poll_interval",
		Type:        core.ConnectionTypeString,
		Label:       "Poll Interval (seconds)",
		Placeholder: "60",
	},
	{
		Name:  "event_types",
		Type:  core.ConnectionTypeString,
		Label: "Event Types",
		Options: []core.ConnectionOption{
			{Name: "New Files", Value: "new"},
			{Name: "Modified Files", Value: "modified"},
			{Name: "Deleted Files", Value: "deleted"},
			{Name: "New & Modified", Value: "new,modified"},
			{Name: "All Events", Value: "new,modified,deleted"},
		},
	},
	{
		Name:        "mime_type_filter",
		Type:        core.ConnectionTypeString,
		Label:       "MIME Type Filter",
		Placeholder: "application/vnd.google-apps.spreadsheet",
	},
}

var Outputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "File Name"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME Type"},
	{Name: "size", Type: core.ConnectionTypeString, Label: "File Size"},
	{Name: "modified_time", Type: core.ConnectionTypeString, Label: "Modified Time"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "web_link", Type: core.ConnectionTypeString, Label: "Web Link"},
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Folder ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Google Drive trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
