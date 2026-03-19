package sql_query

import (
	"database/sql"
	"fmt"

	core "flomation.app/automate/executor"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "PostgreSQL Query"
	Description  = "Perform a PostgreSQL Query"
	Website      = "https://www.flomation.co"
	Icon         = "database"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "driver",
		Type:        core.ConnectionTypeString,
		Label:       "Database Driver",
		Placeholder: "",
	},
	core.Connection{
		Name:        "host",
		Type:        core.ConnectionTypeString,
		Label:       "Database Host",
		Placeholder: "",
	},
	core.Connection{
		Name:        "port",
		Type:        core.ConnectionTypeInteger,
		Label:       "Database Port",
		Placeholder: "",
	},
	core.Connection{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Database Username",
		Placeholder: "",
	},
	core.Connection{
		Name:        "password",
		Type:        core.ConnectionTypeString,
		Label:       "Database Password",
		Placeholder: "",
	},
	core.Connection{
		Name:        "database",
		Type:        core.ConnectionTypeString,
		Label:       "Database Name",
		Placeholder: "",
	},
	core.Connection{
		Name:        "query",
		Type:        core.ConnectionTypeString,
		Label:       "SQL Query",
		Placeholder: "",
	},
	core.Connection{
		Name:        "ssl_mode",
		Type:        core.ConnectionTypeBoolean,
		Label:       "SSL Mode",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	core.Connection{
		Name:        "results",
		Type:        core.ConnectionTypeObject,
		Label:       "Results",
		Placeholder: "",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	driver := core.FindConnection("driver", inputs)
	host := core.FindConnection("host", inputs)
	username := core.FindConnection("username", inputs)
	password := core.FindConnection("password", inputs)
	port := core.FindConnection("port", inputs)
	database := core.FindConnection("database", inputs)
	query := core.FindConnection("query", inputs)
	sslMode := core.FindConnection("ssl_mode", inputs)
	ssl := "disable"

	if sslMode != nil {
		switch *sslMode.String() {
		case "disable":
		case "allow":
		case "prefer":
		case "require":
		case "verify-ca":
		case "verify-full":
			ssl = *sslMode.String()
		default:
			ssl = "disable"
		}
	}

	dsn := fmt.Sprintf("%v://%v:%v@%v:%v/%v?sslmode=%v", *driver.String(), *username.String(), *password.String(), *host.String(), *port.Number(), *database.String(), ssl)
	log.WithFields(log.Fields{
		"dsn": dsn,
	}).Info("Connection String")

	db, err := sql.Open(*driver.String(), dsn)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = db.Close()
	}()

	result, err := db.Exec(*query.String())
	if err != nil {
		return nil, err
	}

	fmt.Printf("%v\n", result)

	return map[string]interface{}{
		"results": nil,
	}, nil
}
