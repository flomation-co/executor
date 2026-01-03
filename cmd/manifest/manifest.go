package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type ManifestEntry struct {
	Author       string
	Organisation string
	Name         string
	Description  string
	Website      string
	Icon         string
	Date         string
}

func inspectPackage(dir string) map[string]ManifestEntry {

	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, nil, 0)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to parse directory")
		return nil
	}

	manifest := make(map[string]ManifestEntry)

	pwd, _ := os.Getwd()
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			diff, _ := filepath.Rel(path.Join(pwd, "actions"), dir)

			var me ManifestEntry
			var meUpdated bool

			for _, object := range f.Decls {
				g, ok := object.(*ast.GenDecl)
				if !ok {
					continue
				}

				for _, s := range g.Specs {
					v, ok := s.(*ast.ValueSpec)
					if !ok {
						continue
					}

					val, ok := v.Values[0].(*ast.BasicLit)
					if !ok {
						continue
					}

					for _, name := range v.Names {
						stringVal, _ := strconv.Unquote(val.Value)

						switch name.String() {
						case "Author":
							me.Author = stringVal
							meUpdated = true
						case "Organisation":
							me.Organisation = stringVal
							meUpdated = true
						case "Name":
							me.Name = stringVal
							meUpdated = true
						case "Description":
							me.Description = stringVal
							meUpdated = true
						case "Website":
							me.Website = stringVal
							meUpdated = true
						case "Icon":
							me.Icon = stringVal
							meUpdated = true
						case "Date":
							me.Date = stringVal
							meUpdated = true
						}
					}
				}
			}

			if diff != "." && meUpdated {
				manifest[diff] = me
			}
		}
	}

	return manifest
}

func parseDir(dir string) (map[string]ManifestEntry, error) {
	fi, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	manifest := inspectPackage(dir)

	for _, d := range fi {
		if d.IsDir() {
			me, err := parseDir(path.Join(dir, d.Name()))
			if err != nil {
				return nil, err
			}

			for k, v := range me {
				manifest[k] = v
			}
		}
	}

	return manifest, nil
}

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to get working directory")
		return
	}

	dir := path.Join(pwd, "actions")
	manifest, err := parseDir(dir)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to parse directory")
	}

	for k, v := range manifest {
		log.WithFields(log.Fields{
			k: v,
		}).Info("Manifest")
	}
}
