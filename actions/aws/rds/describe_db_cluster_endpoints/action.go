// Package aws_rds_describe_db_cluster_endpoints lists Aurora DB cluster endpoints.
package aws_rds_describe_db_cluster_endpoints

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe DB Cluster Endpoints"
	Description  = "List Aurora DB cluster endpoints, optionally filtered by cluster or endpoint."
	Website      = "https://www.flomation.co"
	Icon         = "link+magnifying-glass"
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
	{Name: "db_cluster_identifier", Type: core.ConnectionTypeString, Label: "DB Cluster Identifier (optional)", Placeholder: "Leave blank to list all"},
	{Name: "db_cluster_endpoint_identifier", Type: core.ConnectionTypeString, Label: "Endpoint Identifier (optional)", Placeholder: "Leave blank to list all"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoints", Type: core.ConnectionTypeObject, Label: "Endpoints"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBClusterEndpointsInput{}
	if v := awscommon.InputString("db_cluster_identifier", inputs); v != "" {
		in.DBClusterIdentifier = aws.String(v)
	}
	if v := awscommon.InputString("db_cluster_endpoint_identifier", inputs); v != "" {
		in.DBClusterEndpointIdentifier = aws.String(v)
	}

	var endpoints []map[string]interface{}
	paginator := rds.NewDescribeDBClusterEndpointsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DBClusterEndpoints {
			endpoints = append(endpoints, flattenEndpoint(&page.DBClusterEndpoints[i]))
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d DB cluster endpoint(s)", len(endpoints)),
		"endpoints":   endpoints,
		"count":       len(endpoints),
	}, nil
}

func flattenEndpoint(e *rdstypes.DBClusterEndpoint) map[string]interface{} {
	if e == nil {
		return nil
	}
	return map[string]interface{}{
		"db_cluster_endpoint_identifier": aws.ToString(e.DBClusterEndpointIdentifier),
		"db_cluster_identifier":          aws.ToString(e.DBClusterIdentifier),
		"endpoint":                       aws.ToString(e.Endpoint),
		"status":                         aws.ToString(e.Status),
		"endpoint_type":                  aws.ToString(e.EndpointType),
		"custom_endpoint_type":           aws.ToString(e.CustomEndpointType),
		"static_members":                 e.StaticMembers,
		"excluded_members":               e.ExcludedMembers,
	}
}
