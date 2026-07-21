// Package aws_rds_restore_db_instance_from_s3 creates a new RDS DB instance from
// a database backup stored in Amazon S3.
package aws_rds_restore_db_instance_from_s3

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
	Name         = "AWS RDS Restore DB Instance From S3"
	Description  = "Create a new RDS DB instance from a database backup stored in S3."
	Website      = "https://www.flomation.co"
	Icon         = "database+arrow-up"
	Date         = "21/07/2026"
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
	{Name: "engine", Type: core.ConnectionTypeString, Label: "Engine", Placeholder: "mysql", Required: true},
	{Name: "master_username", Type: core.ConnectionTypeString, Label: "Master Username", Placeholder: "admin", Required: true},
	{Name: "master_password", Type: core.ConnectionTypeSecret, Label: "Master Password", Required: true},
	{Name: "allocated_storage", Type: core.ConnectionTypeInteger, Label: "Allocated Storage (GiB)", Placeholder: "20", Required: true},
	{Name: "s3_bucket_name", Type: core.ConnectionTypeString, Label: "S3 Bucket Name", Placeholder: "my-backup-bucket", Required: true},
	{Name: "s3_ingestion_role_arn", Type: core.ConnectionTypeString, Label: "S3 Ingestion Role ARN", Placeholder: "arn:aws:iam::<account>:role/rds-s3-import", Required: true},
	{Name: "source_engine", Type: core.ConnectionTypeString, Label: "Source Engine", Placeholder: "mysql", Required: true},
	{Name: "source_engine_version", Type: core.ConnectionTypeString, Label: "Source Engine Version", Placeholder: "e.g. 8.0.36", Required: true},
	{Name: "s3_prefix", Type: core.ConnectionTypeString, Label: "S3 Prefix (optional)", Placeholder: "Path within the bucket to the backup files"},
	{Name: "db_subnet_group_name", Type: core.ConnectionTypeString, Label: "DB Subnet Group (optional)", Placeholder: "Places the DB in a specific VPC"},
	{Name: "vpc_security_group_ids", Type: core.ConnectionTypeString, Label: "VPC Security Group IDs (optional)", Placeholder: "Comma-separated, e.g. sg-0abc,sg-0def"},
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
	bucket := awscommon.InputString("s3_bucket_name", inputs)
	roleArn := awscommon.InputString("s3_ingestion_role_arn", inputs)
	sourceEngine := awscommon.InputString("source_engine", inputs)
	sourceEngineVersion := awscommon.InputString("source_engine_version", inputs)
	storage, hasStorage := awscommon.InputInt("allocated_storage", inputs)
	if id == "" || class == "" || engine == "" || username == "" || password == "" ||
		bucket == "" || roleArn == "" || sourceEngine == "" || sourceEngineVersion == "" || !hasStorage {
		return nil, fmt.Errorf("db instance identifier, instance class, engine, master username, master password, allocated storage, S3 bucket, S3 ingestion role ARN, source engine and source engine version are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.RestoreDBInstanceFromS3Input{
		DBInstanceIdentifier: aws.String(id),
		DBInstanceClass:      aws.String(class),
		Engine:               aws.String(engine),
		MasterUsername:       aws.String(username),
		MasterUserPassword:   aws.String(password),
		AllocatedStorage:     aws.Int32(int32(storage)),
		S3BucketName:         aws.String(bucket),
		S3IngestionRoleArn:   aws.String(roleArn),
		SourceEngine:         aws.String(sourceEngine),
		SourceEngineVersion:  aws.String(sourceEngineVersion),
	}
	if v := awscommon.InputString("s3_prefix", inputs); v != "" {
		in.S3Prefix = aws.String(v)
	}
	if v := awscommon.InputString("db_subnet_group_name", inputs); v != "" {
		in.DBSubnetGroupName = aws.String(v)
	}
	if ids := awscommon.InputStrings("vpc_security_group_ids", inputs); len(ids) > 0 {
		in.VpcSecurityGroupIds = ids
	}
	if awscommon.InputBool("publicly_accessible", inputs) {
		in.PubliclyAccessible = aws.Bool(true)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.RestoreDBInstanceFromS3(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Restoring DB instance %q from s3://%s (%s, status: %s)", id, bucket, engine, aws.ToString(out.DBInstance.DBInstanceStatus)),
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
