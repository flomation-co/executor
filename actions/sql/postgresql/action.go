package sql_postgresql

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	sql_common "flomation.app/automate/executor/actions/sql"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "PostgreSQL Query"
	Description  = "Execute a query against a PostgreSQL database"
	Website      = "https://www.flomation.co"
	Icon         = "database"
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
		Placeholder: "5432",
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
		Name:        "database",
		Type:        core.ConnectionTypeString,
		Label:       "Database Name",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "query",
		Type:        core.ConnectionTypeText,
		Label:       "SQL Query",
		Placeholder: "SELECT * FROM ...",
		Required:    true,
	},
	{
		Name:        "ssl_mode",
		Type:        core.ConnectionTypeString,
		Label:       "SSL Mode",
		Placeholder: "",
		Options: []core.ConnectionOption{
			{Name: "Disable", Value: "disable"},
			{Name: "Allow", Value: "allow"},
			{Name: "Prefer", Value: "prefer"},
			{Name: "Require", Value: "require"},
			{Name: "Verify CA", Value: "verify-ca"},
			{Name: "Verify Full", Value: "verify-full"},
		},
	},
}

var Outputs = [...]core.Connection{
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
	database := core.FindConnection("database", inputs)
	query := core.FindConnection("query", inputs)
	sslModeConn := core.FindConnection("ssl_mode", inputs)

	sslMode := "disable"
	if sslModeConn != nil && sslModeConn.String() != nil {
		val := *sslModeConn.String()
		switch val {
		case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
			sslMode = val
		}
	}

	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%v/%s?sslmode=%s",
		url.QueryEscape(*username.String()),
		url.QueryEscape(*password.String()),
		*host.String(),
		*port.Number(),
		*database.String(),
		sslMode,
	)

	log.WithFields(log.Fields{
		"host":     *host.String(),
		"port":     *port.Number(),
		"database": *database.String(),
		"ssl_mode": sslMode,
	}).Info("Connecting to PostgreSQL")

	db, err := sql_common.OpenConnection("postgres", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return sql_common.ExecuteQuery(db, *query.String())
}
