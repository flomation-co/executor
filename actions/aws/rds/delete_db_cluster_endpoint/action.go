// Package aws_rds_delete_db_cluster_endpoint deletes a custom Aurora DB cluster endpoint.
package aws_rds_delete_db_cluster_endpoint

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Delete DB Cluster Endpoint"
	Description  = "Delete a custom Aurora DB cluster endpoint."
	Website      = "https://www.flomation.co"
	Icon         = "link+trash"
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
	{Name: "db_cluster_endpoint_identifier", Type: core.ConnectionTypeString, Label: "Endpoint Identifier", Placeholder: "my-custom-endpoint", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoint", Type: core.ConnectionTypeObject, Label: "Endpoint"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	endpointID := awscommon.InputString("db_cluster_endpoint_identifier", inputs)
	if endpointID == "" {
		return nil, fmt.Errorf("endpoint identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	out, err := client.DeleteDBClusterEndpoint(ctx, &rds.DeleteDBClusterEndpointInput{
		DBClusterEndpointIdentifier: aws.String(endpointID),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleting DB cluster endpoint %q (status: %s)", endpointID, aws.ToString(out.Status)),
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
