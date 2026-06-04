package sql_oracle

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	sql_common "flomation.app/automate/executor/actions/sql"
	_ "github.com/sijms/go-ora/v2"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Oracle Query"
	Description  = "Execute a query against an Oracle database"
	Website      = "https://www.flomation.co"
	Icon         = "database+play"
	Date         = "21/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "host",
		Type:        core.ConnectionTypeString,
		Label:       "Database Host",
		Placeholder: "localhost",
		Required:    true,
	},
	{
		Name:        "port",
		Type:        core.ConnectionTypeInteger,
		Label:       "Database Port",
		Placeholder: "1521",
		Required:    true,
	},
	{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "password",
		Type:        core.ConnectionTypeString,
		Label:       "Password",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "service_name",
		Type:        core.ConnectionTypeString,
		Label:       "Service Name",
		Placeholder: "ORCL",
		Required:    true,
	},
	{
		Name:        "query",
		Type:        core.ConnectionTypeText,
		Label:       "SQL Query",
		Placeholder: "SELECT * FROM ...",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{
		Name:  "results",
		Type:  core.ConnectionTypeObject,
		Label: "Results",
	},
	{
		Name:  "row_count",
		Type:  core.ConnectionTypeInteger,
		Label: "Row Count",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host := core.FindConnection("host", inputs)
	port := core.FindConnection("port", inputs)
	username := core.FindConnection("username", inputs)
	password := core.FindConnection("password", inputs)
	serviceName := core.FindConnection("service_name", inputs)
	query := core.FindConnection("query", inputs)

	dsn := fmt.Sprintf("oracle://%s:%s@%s:%v/%s",
		url.QueryEscape(*username.String()),
		url.QueryEscape(*password.String()),
		*host.String(),
		*port.Number(),
		*serviceName.String(),
	)

	log.WithFields(log.Fields{
		"host":         *host.String(),
		"port":         *port.Number(),
		"service_name": *serviceName.String(),
	}).Info("Connecting to Oracle")

	db, err := sql_common.OpenConnection("oracle", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return sql_common.ExecuteQuery(db, *query.String())
}
