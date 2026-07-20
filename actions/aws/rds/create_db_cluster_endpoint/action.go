// Package aws_rds_create_db_cluster_endpoint creates a custom Aurora DB cluster endpoint.
package aws_rds_create_db_cluster_endpoint

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create DB Cluster Endpoint"
	Description  = "Create a custom Aurora DB cluster endpoint (reader or any)."
	Website      = "https://www.flomation.co"
	Icon         = "link+plus"
	Date         = "20/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
		{Name: "Managed Role (Credential)", Value: "credential"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "db_cluster_identifier", Type: core.ConnectionTypeString, Label: "DB Cluster Identifier", Placeholder: "my-cluster", Required: true},
	{Name: "db_cluster_endpoint_identifier", Type: core.ConnectionTypeString, Label: "Endpoint Identifier", Placeholder: "my-custom-endpoint", Required: true},
	{Name: "endpoint_type", Type: core.ConnectionTypeString, Label: "Endpoint Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Reader", Value: "READER"},
		{Name: "Any", Value: "ANY"},
	}},
	{Name: "static_members", Type: core.ConnectionTypeString, Label: "Static Members (optional)", Placeholder: "Comma-separated DB instance identifiers"},
	{Name: "excluded_members", Type: core.ConnectionTypeString, Label: "Excluded Members (optional)", Placeholder: "Comma-separated DB instance identifiers"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoint", Type: core.ConnectionTypeObject, Label: "Endpoint"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cluster := awscommon.InputString("db_cluster_identifier", inputs)
	endpointID := awscommon.InputString("db_cluster_endpoint_identifier", inputs)
	endpointType := awscommon.InputString("endpoint_type", inputs)
	if cluster == "" || endpointID == "" || endpointType == "" {
		return nil, fmt.Errorf("db cluster identifier, endpoint identifier and endpoint type are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateDBClusterEndpointInput{
		DBClusterIdentifier:         aws.String(cluster),
		DBClusterEndpointIdentifier: aws.String(endpointID),
		EndpointType:                aws.String(endpointType),
	}
	if v := awscommon.InputStrings("static_members", inputs); len(v) > 0 {
		in.StaticMembers = v
	}
	if v := awscommon.InputStrings("excluded_members", inputs); len(v) > 0 {
		in.ExcludedMembers = v
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateDBClusterEndpoint(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Creating DB cluster endpoint %q on cluster %q (status: %s)", endpointID, cluster, aws.ToString(out.Status)),
		"endpoint": map[string]interface{}{
			"db_cluster_endpoint_identifier": aws.ToString(out.DBClusterEndpointIdentifier),
			"db_cluster_identifier":          aws.ToString(out.DBClusterIdentifier),
			"endpoint":                       aws.ToString(out.Endpoint),
			"status":                         aws.ToString(out.Status),
			"endpoint_type":                  aws.ToString(out.EndpointType),
			"custom_endpoint_type":           aws.ToString(out.CustomEndpointType),
			"static_members":                 out.StaticMembers,
			"excluded_members":               out.ExcludedMembers,
		},
	}, nil
}

func buildTags(inputs []*core.Connection) []rdstypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []rdstypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, rdstypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
