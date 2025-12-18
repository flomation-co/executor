package main

import (
	"flag"
	"strings"
	"time"

	"flomation.app/automate/executor/internal/environment"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions"
	"flomation.app/automate/executor/internal/version"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	LogFormatDefault = "default"
	LogFormatJSON    = "json"
)

const (
	DefaultAPI              = "https://api.flomation.app"
	DefaultEnvironment      = "default"
	DefaultRunnerIdentifier = "local"
)

func main() {
	log.SetLevel(log.DebugLevel)

	log.WithFields(log.Fields{
		"version": version.Version,
		"hash":    version.GetHash(),
		"date":    version.BuiltDate,
	}).Info("Starting Flomation Executor")

	path := flag.String("path", "", "Path of the Flow file to execute")
	entry := flag.String("entry", "", "Entry node to begin execution")
	id := flag.String("id", uuid.NewString(), "Execution ID")
	api := flag.String("api", DefaultAPI, "URL for API Service")
	env := flag.String("environment", DefaultEnvironment, "ID of environment to execute within")
	runner := flag.String("runner", DefaultRunnerIdentifier, "Runner ID")
	logOutput := flag.String("output", LogFormatDefault, "Log output format - default/json")
	user := flag.String("user", "", "Execution context username")
	password := flag.String("password", "", "Execution context username")
	token := flag.String("token", "", "Execution context credential token")
	identity := flag.String("identity", "https://id.flomation.app", "URL of Identity Service Provider")

	flag.Parse()
	if strings.ToLower(*logOutput) == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if *path == "" {
		log.Error("no path specified")
		flag.PrintDefaults()
		return
	}

	log.WithFields(log.Fields{
		"path":        *path,
		"entry":       *entry,
		"id":          *id,
		"api":         *api,
		"environment": *env,
		"runner":      *runner,
	}).Info("Executing Flow")

	flo, err := core.Load(path)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Unable to load flow")
		return
	}

	var e *environment.Environment
	if *env != DefaultEnvironment {
		var auth *environment.Credentials
		if *user != "" && *password != "" {
			auth = environment.Authenticate(*user, *password, identity)
		}

		if *token != "" {
			auth = environment.Token(*token)
		}

		e, err = environment.NewEnvironment(*env, api, auth)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Unable to configure environment")
			return
		}
	}

	var entryNode *string
	if *entry != "" {
		entryNode = entry
	}

	start := time.Now()
	outputs, err := flo.Execute(actions.Actions, entryNode, e)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Error executing flow")
	}

	duration := time.Since(start)

	log.WithFields(log.Fields{
		"outputs":     outputs,
		"duration_ms": duration.Milliseconds(),
	}).Info("Finished processing Flow")
}
