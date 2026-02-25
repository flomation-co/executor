package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	core "flomation.app/automate/executor"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type ManifestEntry struct {
	Hash string `json:"hash"`

	Author       string `json:"author"`
	Organisation string `json:"organisation"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Website      string `json:"website"`
	Icon         string `json:"icon"`
	Date         string `json:"date"`

	Inputs  []core.Connection `json:"inputs"`
	Outputs []core.Connection `json:"outputs"`
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
		diff, _ := filepath.Rel(path.Join(pwd, "actions"), dir)

		de, err := os.ReadDir(path.Join(pwd, "actions", diff))
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("unable to read directory")
		}

		var me ManifestEntry
		var meUpdated bool

		if de != nil {
			h := sha256.New()
			for _, f := range de {
				if f.IsDir() {
					continue
				}

				filePath := path.Join(pwd, "actions", diff, f.Name())

				f, err := os.Open(filePath)
				if err != nil {
					log.WithFields(log.Fields{
						"filepath": filePath,
						"error":    err,
					}).Error("unable to open file")
					continue
				}
				_, err = io.Copy(h, f)
				if err != nil {
					log.WithFields(log.Fields{
						"filepath": filePath,
						"error":    err,
					}).Error("unable to copy file contents to hash")
					continue
				}
			}

			hash := h.Sum(nil)
			me.Hash = string(hex.EncodeToString(hash))
		}

		for _, f := range pkg.Files {
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
						val, ok := v.Values[0].(*ast.CompositeLit)
						if !ok {
							continue
						}

						name := v.Names[0].Name
						isInput := true
						if name != "Inputs" && name != "Outputs" {
							continue
						}

						if name == "Inputs" {
							isInput = true
						} else if name == "Outputs" {
							isInput = false
						}

						var connections []core.Connection

						for _, e := range val.Elts {
							lit, ok := e.(*ast.CompositeLit)
							if !ok {
								continue
							}

							var c core.Connection

							for _, e := range lit.Elts {
								el, ok := e.(*ast.KeyValueExpr)
								if !ok {
									continue
								}

								var value string
								key := el.Key.(*ast.Ident)

								connectionValue, ok := el.Value.(*ast.BasicLit)
								if !ok {
									selector, ok := el.Value.(*ast.SelectorExpr)
									if !ok {
										continue
									}

									t := selector.Sel.Name

									switch t {
									case "ConnectionTypeString":
										value = "string"
									case "ConnectionTypeObject":
										value = "object"
									case "ConnectionTypeInteger":
										value = "integer"
									case "ConnectionTypeBoolean":
										value = "boolean"
									}
								} else {
									value, _ = strconv.Unquote(connectionValue.Value)
								}

								switch key.Name {
								case "Name":
									c.Name = value
								case "Label":
									c.Label = value
								case "Placeholder":
									c.Placeholder = value
								case "Type":
									c.Type = value
								}
							}

							connections = append(connections, c)
						}

						if isInput {
							me.Inputs = connections
						} else {
							me.Outputs = connections
						}

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
		}

		if diff != "." && meUpdated {
			manifest[diff] = me
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

	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to marshal json manifest")
	}

	if err := os.WriteFile("actions-manifest.json", b, 0600); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to write manifest file")
	}
}
