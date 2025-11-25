package main

import (
	"flag"
	"flomation.app/automate/executor/internal/version"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.WithFields(log.Fields{
		"version": version.Version,
		"hash":    version.GetHash(),
		"date":    version.BuiltDate,
	}).Info("Starting Flomation Executor")

	path := flag.String("path", "", "Path of the Flow file to execute")
	entry := flag.String("entry", "", "Entry node to begin execution")
	id := flag.String("id", uuid.NewString(), "Execution ID")
	api := flag.String("api", "https://api.flomation.app", "URL for API Service")
	environment := flag.String("environment", "", "ID of environment to execute within")
	runner := flag.String("runner", "", "Runner ID")

	flag.Parse()
	if *path == "" {
		log.Error("no path specified")
		flag.PrintDefaults()
		return
	}

	log.WithFields(log.Fields{
		"path":        path,
		"entry":       entry,
		"id":          id,
		"api":         api,
		"environment": environment,
		"runner":      runner,
	}).Info("executing Flow")
}
