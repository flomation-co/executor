// Package aws_rds_create_db_instance provisions a new RDS database instance.
package aws_rds_create_db_instance

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	rdscat "flomation.app/automate/executor/actions/aws/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create DB Instance"
	Description  = "Provision a new RDS database instance."
	Website      = "https://www.flomation.co"
	Icon         = "database+plus"
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
	{Name: "db_instance_identifier", Type: core.ConnectionTypeString, Label: "DB Instance Identifier", Placeholder: "my-database", Required: true},
	{Name: "db_instance_class", Type: core.ConnectionTypeString, Label: "Instance Class", Placeholder: "db.t3.micro", Required: true},
	{Name: "engine", Type: core.ConnectionTypeString, Label: "Engine", Required: true, Options: []core.ConnectionOption{
		{Name: "PostgreSQL", Value: "postgres"},
		{Name: "MySQL", Value: "mysql"},
		{Name: "MariaDB", Value: "mariadb"},
		{Name: "Oracle (SE2)", Value: "oracle-se2"},
		{Name: "SQL Server (Express)", Value: "sqlserver-ex"},
	}},
	{Name: "allocated_storage", Type: core.ConnectionTypeInteger, Label: "Allocated Storage (GiB)", Placeholder: "20", Required: true},
	{Name: "master_username", Type: core.ConnectionTypeString, Label: "Master Username", Placeholder: "admin", Required: true},
	{Name: "master_password", Type: core.ConnectionTypeSecret, Label: "Master Password", Required: true},
	{Name: "db_name", Type: core.ConnectionTypeString, Label: "Initial Database Name (optional)"},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "Engine Version (optional)", Placeholder: "e.g. 16.3"},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port (optional)", Placeholder: "Engine default"},
	{Name: "storage_type", Type: core.ConnectionTypeString, Label: "Storage Type (optional)", Options: []core.ConnectionOption{
		{Name: "General Purpose SSD (gp2)", Value: "gp2"},
		{Name: "General Purpose SSD (gp3)", Value: "gp3"},
		{Name: "Provisioned IOPS (io1)", Value: "io1"},
		{Name: "Magnetic (standard)", Value: "standard"},
	}},
	{Name: "publicly_accessible", Type: core.ConnectionTypeBoolean, Label: "Publicly Accessible"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance", Type: core.ConnectionTypeObject, Label: "DB Instance"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("db_instance_identifier", inputs)
	class := awscommon.InputString("db_instance_class", inputs)
	engine := awscommon.InputString("engine", inputs)
	username := awscommon.InputString("master_username", inputs)
	password := awscommon.InputString("master_password", inputs)
	storage, hasStorage := awscommon.InputInt("allocated_storage", inputs)
	if id == "" || class == "" || engine == "" || username == "" || password == "" || !hasStorage {
		return nil, fmt.Errorf("identifier, instance class, engine, master username, master password and allocated storage are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(id),
		DBInstanceClass:      aws.String(class),
		Engine:               aws.String(engine),
		MasterUsername:       aws.String(username),
		MasterUserPassword:   aws.String(password),
		AllocatedStorage:     aws.Int32(int32(storage)),
	}
	if v := awscommon.InputString("db_name", inputs); v != "" {
		in.DBName = aws.String(v)
	}
	if v := awscommon.InputString("engine_version", inputs); v != "" {
		in.EngineVersion = aws.String(v)
	}
	if v := strings.TrimSpace(awscommon.InputString("storage_type", inputs)); v != "" {
		in.StorageType = aws.String(v)
	}
	if p, ok := awscommon.InputInt("port", inputs); ok {
		in.Port = aws.Int32(int32(p))
	}
	if awscommon.InputBool("publicly_accessible", inputs) {
		in.PubliclyAccessible = aws.Bool(true)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateDBInstance(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Creating DB instance %q (%s, %s, status: %s)", id, engine, class, aws.ToString(out.DBInstance.DBInstanceStatus)),
		"instance":    rdscat.SummariseInstance(out.DBInstance),
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
