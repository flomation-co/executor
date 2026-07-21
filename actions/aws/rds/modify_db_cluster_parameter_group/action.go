// Package aws_rds_modify_db_cluster_parameter_group modifies parameters in an RDS DB cluster parameter group.
package aws_rds_modify_db_cluster_parameter_group

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
	Name         = "AWS RDS Modify DB Cluster Parameter Group"
	Description  = "Modify parameter values in an RDS DB cluster parameter group."
	Website      = "https://www.flomation.co"
	Icon         = "gears+pen"
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
	{Name: "db_cluster_parameter_group_name", Type: core.ConnectionTypeString, Label: "Cluster Parameter Group Name", Placeholder: "my-cpg-aurora-postgresql16", Required: true},
	{Name: "parameters", Type: core.ConnectionTypeKeyValueArray, Label: "Parameters (name = value)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "applied", Type: core.ConnectionTypeInteger, Label: "Parameters Applied"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("db_cluster_parameter_group_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("cluster parameter group name is required")
	}

	params := buildParameters(inputs)
	if len(params) == 0 {
		return nil, fmt.Errorf("at least one parameter is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	_, err = client.ModifyDBClusterParameterGroup(ctx, &rds.ModifyDBClusterParameterGroupInput{
		DBClusterParameterGroupName: aws.String(name),
		Parameters:                  params,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified %d parameter(s) in DB cluster parameter group %q", len(params), name),
		"applied":     len(params),
	}, nil
}

func buildParameters(inputs []*core.Connection) []rdstypes.Parameter {
	conn := core.FindConnection("parameters", inputs)
	if conn == nil {
		return nil
	}
	var params []rdstypes.Parameter
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		params = append(params, rdstypes.Parameter{
			ParameterName:  aws.String(k),
			ParameterValue: aws.String(kv.Value),
			ApplyMethod:    rdstypes.ApplyMethodImmediate,
		})
	}
	return params
}
