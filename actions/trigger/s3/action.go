package s3

import (
	core "flomation.app/automate/executor"

	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "S3 Trigger"
	Description  = "Triggers a flow when objects are created or deleted in an S3 bucket"
	Website      = "https://www.flomation.co"
	Icon         = "bucket"
	Date         = "23/03/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{
		Name:        "bucket_name",
		Type:        core.ConnectionTypeString,
		Label:       "Bucket Name",
		Placeholder: "my-bucket",
		Required:    true,
	},
	{
		Name:        "prefix",
		Type:        core.ConnectionTypeString,
		Label:       "Key Prefix",
		Placeholder: "Optional key prefix filter",
	},
	{
		Name:        "aws_access_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "AWS Access Key",
		Placeholder: "AKIA...",
		Required:    true,
	},
	{
		Name:        "aws_secret_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "AWS Secret Key",
		Placeholder: "Secret key for authentication",
		Required:    true,
	},
	{
		Name:     "region",
		Type:     core.ConnectionTypeSecret,
		Label:    "AWS Region",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "US East (N. Virginia)", Value: "us-east-1"},
			{Name: "US East (Ohio)", Value: "us-east-2"},
			{Name: "US West (N. California)", Value: "us-west-1"},
			{Name: "US West (Oregon)", Value: "us-west-2"},
			{Name: "EU (Ireland)", Value: "eu-west-1"},
			{Name: "EU (London)", Value: "eu-west-2"},
			{Name: "EU (Frankfurt)", Value: "eu-central-1"},
			{Name: "Asia Pacific (Singapore)", Value: "ap-southeast-1"},
			{Name: "Asia Pacific (Sydney)", Value: "ap-southeast-2"},
			{Name: "Asia Pacific (Tokyo)", Value: "ap-northeast-1"},
			{Name: "Asia Pacific (Seoul)", Value: "ap-northeast-2"},
			{Name: "Asia Pacific (Mumbai)", Value: "ap-south-1"},
			{Name: "Canada (Central)", Value: "ca-central-1"},
			{Name: "South America (São Paulo)", Value: "sa-east-1"},
		},
	},
	{
		Name:        "poll_interval",
		Type:        core.ConnectionTypeString,
		Label:       "Poll Interval",
		Placeholder: "e.g. 60s, 5m",
	},
	{
		Name:  "event_types",
		Type:  core.ConnectionTypeString,
		Label: "Event Types",
		Options: []core.ConnectionOption{
			{Name: "Put", Value: "put"},
			{Name: "Delete", Value: "delete"},
			{Name: "Put & Delete", Value: "put,delete"},
		},
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "bucket",
		Type:  core.ConnectionTypeString,
		Label: "Bucket",
	},
	{
		Name:  "key",
		Type:  core.ConnectionTypeString,
		Label: "Object Key",
	},
	{
		Name:  "size",
		Type:  core.ConnectionTypeInteger,
		Label: "Size",
	},
	{
		Name:  "last_modified",
		Type:  core.ConnectionTypeString,
		Label: "Last Modified",
	},
	{
		Name:  "etag",
		Type:  core.ConnectionTypeString,
		Label: "ETag",
	},
	{
		Name:  "event_type",
		Type:  core.ConnectionTypeString,
		Label: "Event Type",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing S3 trigger")

	result := make(map[string]interface{})

	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
