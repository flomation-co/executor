package sql_mysql

import (
	"fmt"

	core "flomation.app/automate/executor"
	sql_common "flomation.app/automate/executor/actions/sql"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "MySQL Query"
	Description  = "Execute a query against a MySQL database"
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
		Placeholder: "3306",
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
		Name:        "tls_mode",
		Type:        core.ConnectionTypeString,
		Label:       "TLS Mode",
		Placeholder: "",
		Options: []core.ConnectionOption{
			{Name: "Disabled", Value: "false"},
			{Name: "Preferred", Value: "preferred"},
			{Name: "Required", Value: "true"},
			{Name: "Skip Verify", Value: "skip-verify"},
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
	tlsModeConn := core.FindConnection("tls_mode", inputs)

	tlsMode := "false"
	if tlsModeConn != nil && tlsModeConn.String() != nil {
		val := *tlsModeConn.String()
		switch val {
		case "false", "preferred", "true", "skip-verify":
			tlsMode = val
		}
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%v)/%s?tls=%s",
		*username.String(),
		*password.String(),
		*host.String(),
		*port.Number(),
		*database.String(),
		tlsMode,
	)

	log.WithFields(log.Fields{
		"host":     *host.String(),
		"port":     *port.Number(),
		"database": *database.String(),
		"tls_mode": tlsMode,
	}).Info("Connecting to MySQL")

	db, err := sql_common.OpenConnection("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return sql_common.ExecuteQuery(db, *query.String())
}
