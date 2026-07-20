// Package aws_rds_describe_global_clusters lists Aurora global database clusters.
package aws_rds_describe_global_clusters

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
	Name         = "AWS RDS Describe Global Clusters"
	Description  = "List Aurora global database clusters, optionally filtered by identifier."
	Website      = "https://www.flomation.co"
	Icon         = "globe+magnifying-glass"
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
	{Name: "global_cluster_identifier", Type: core.ConnectionTypeString, Label: "Global Cluster Identifier (optional)", Placeholder: "Leave blank to list all"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "global_clusters", Type: core.ConnectionTypeObject, Label: "Global Clusters"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeGlobalClustersInput{}
	if id := awscommon.InputString("global_cluster_identifier", inputs); id != "" {
		in.GlobalClusterIdentifier = aws.String(id)
	}

	var clusters []map[string]interface{}
	paginator := rds.NewDescribeGlobalClustersPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.GlobalClusters {
			clusters = append(clusters, flattenGlobalCluster(&page.GlobalClusters[i]))
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Found %d global cluster(s)", len(clusters)),
		"global_clusters": clusters,
		"count":           len(clusters),
	}, nil
}

func flattenGlobalCluster(gc *rdstypes.GlobalCluster) map[string]interface{} {
	if gc == nil {
		return nil
	}
	var members []map[string]interface{}
	for _, m := range gc.GlobalClusterMembers {
		members = append(members, map[string]interface{}{
			"db_cluster_arn": aws.ToString(m.DBClusterArn),
			"is_writer":      aws.ToBool(m.IsWriter),
		})
	}
	return map[string]interface{}{
		"global_cluster_identifier": aws.ToString(gc.GlobalClusterIdentifier),
		"status":                    aws.ToString(gc.Status),
		"engine":                    aws.ToString(gc.Engine),
		"engine_version":            aws.ToString(gc.EngineVersion),
		"members":                   members,
	}
}
