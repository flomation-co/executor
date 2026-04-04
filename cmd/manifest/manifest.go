package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	core "flomation.app/automate/executor"

	log "github.com/sirupsen/logrus"
)

type CategoryMeta struct {
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type ManifestEntry struct {
	Hash string `json:"hash"`

	Author       string `json:"author"`
	Organisation string `json:"organisation"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Website      string `json:"website"`
	Icon         string `json:"icon"`
	Date         string `json:"date"`
	Type         int64  `json:"type"`

	Category    *CategoryMeta `json:"category,omitempty"`
	SubCategory *CategoryMeta `json:"sub_category,omitempty"`

	Inputs  []core.Connection `json:"inputs"`
	Outputs []core.Connection `json:"outputs"`

	HasExecute  bool `json:"-"`
	HasCategory bool `json:"-"`
}

func parseConnectionOptions(compositeLit *ast.CompositeLit) []core.ConnectionOption {
	var options []core.ConnectionOption
	for _, elt := range compositeLit.Elts {
		optLit, ok := elt.(*ast.CompositeLit)
		if !ok {
			continue
		}
		var opt core.ConnectionOption
		for _, field := range optLit.Elts {
			kv, ok := field.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			val, ok := kv.Value.(*ast.BasicLit)
			if !ok {
				continue
			}
			s, _ := strconv.Unquote(val.Value)
			switch key.Name {
			case "Name":
				opt.Name = s
			case "Value":
				opt.Value = s
			}
		}
		options = append(options, opt)
	}
	return options
}

func parseVisibleWhen(compositeLit *ast.CompositeLit) *core.VisibleWhen {
	vw := &core.VisibleWhen{}
	for _, field := range compositeLit.Elts {
		kv, ok := field.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Field":
			if v, ok := kv.Value.(*ast.BasicLit); ok {
				vw.Field, _ = strconv.Unquote(v.Value)
			}
		case "Values":
			if cl, ok := kv.Value.(*ast.CompositeLit); ok {
				for _, elt := range cl.Elts {
					if v, ok := elt.(*ast.BasicLit); ok {
						s, _ := strconv.Unquote(v.Value)
						vw.Values = append(vw.Values, s)
					}
				}
			}
		}
	}
	return vw
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
				if fn, ok := object.(*ast.FuncDecl); ok {
					if fn.Name.Name == "Execute" {
						me.HasExecute = true
						meUpdated = true
					}
					continue
				}

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

						if val, ok := v.Values[0].(*ast.SelectorExpr); ok {
							for _, name := range v.Names {
								switch name.String() {
								case "Type":
									switch val.Sel.Name {
									case "ActionTypeTrigger":
										me.Type = core.ActionTypeTrigger
										meUpdated = true
									case "ActionTypeAction":
										me.Type = core.ActionTypeAction
										meUpdated = true
									case "ActionTypeOutput":
										me.Type = core.ActionTypeOutput
										meUpdated = true
									case "ActionTypeConditional":
										me.Type = core.ActionTypeConditional
										meUpdated = true
									case "ActionTypeLoop":
										me.Type = core.ActionTypeLoop
										meUpdated = true
									case "ActionTypeSwitch":
										me.Type = core.ActionTypeSwitch
										meUpdated = true
									}
								}
							}

							continue
						}

						val, ok := v.Values[0].(*ast.CompositeLit)
						if !ok {
							continue
						}

						name := v.Names[0].Name
						isInput := true
						if name != "Inputs" && name != "Outputs" {
							continue
						}

						switch name {
						case "Inputs":
							isInput = true
						case "Outputs":
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
								var boolValue *bool
								key := el.Key.(*ast.Ident)

								switch v := el.Value.(type) {
								case *ast.BasicLit:
									value, _ = strconv.Unquote(v.Value)
								case *ast.SelectorExpr:
									t := v.Sel.Name
									switch t {
									case "ConnectionTypeString":
										value = "string"
									case "ConnectionTypeObject":
										value = "object"
									case "ConnectionTypeInteger":
										value = "integer"
									case "ConnectionTypeBoolean":
										value = "boolean"
									case "ConnectionTypeText":
										value = "text"
									case "ConnectionTypeKeyValueArray":
										value = "key_value_array"
									}
								case *ast.Ident:
									if v.Name == "true" {
										b := true
										boolValue = &b
									}
									if v.Name == "false" {
										b := false
										boolValue = &b
									}
								case *ast.CompositeLit:
									if key.Name == "Options" {
										c.Options = parseConnectionOptions(v)
									}
								case *ast.UnaryExpr:
									if key.Name == "Visible" {
										if cl, ok := v.X.(*ast.CompositeLit); ok {
											c.Visible = parseVisibleWhen(cl)
										}
									}
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
								case "Required":
									if boolValue != nil {
										c.Required = *boolValue
									}
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
						switch name.String() {
						case "Author":
							stringVal, _ := strconv.Unquote(val.Value)
							me.Author = stringVal
							meUpdated = true
						case "Organisation":
							stringVal, _ := strconv.Unquote(val.Value)
							me.Organisation = stringVal
							meUpdated = true
						case "Name":
							stringVal, _ := strconv.Unquote(val.Value)
							me.Name = stringVal
							meUpdated = true
						case "Description":
							stringVal, _ := strconv.Unquote(val.Value)
							me.Description = stringVal
							meUpdated = true
						case "Website":
							stringVal, _ := strconv.Unquote(val.Value)
							me.Website = stringVal
							meUpdated = true
						case "Icon":
							stringVal, _ := strconv.Unquote(val.Value)
							me.Icon = stringVal
							meUpdated = true
						case "Date":
							stringVal, _ := strconv.Unquote(val.Value)
							me.Date = stringVal
							meUpdated = true
						case "Type":
							me.Type, _ = strconv.ParseInt(val.Value, 10, 64)
							meUpdated = true
						case "CategoryName":
							stringVal, _ := strconv.Unquote(val.Value)
							if me.Category == nil {
								me.Category = &CategoryMeta{}
							}
							me.Category.Name = stringVal
							me.HasCategory = true
						case "CategoryIcon":
							stringVal, _ := strconv.Unquote(val.Value)
							if me.Category == nil {
								me.Category = &CategoryMeta{}
							}
							me.Category.Icon = stringVal
							me.HasCategory = true
						case "CategoryDescription":
							stringVal, _ := strconv.Unquote(val.Value)
							if me.Category == nil {
								me.Category = &CategoryMeta{}
							}
							me.Category.Description = stringVal
							me.HasCategory = true
						}
					}
				}
			}
		}

		if diff != "." && (meUpdated || me.HasCategory) {
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

var actionsGoTemplate = template.Must(template.New("actions").Parse(`// Code generated by cmd/manifest/manifest.go; DO NOT EDIT.

package actions

import (
	core "flomation.app/automate/executor"
{{range .Imports}}
	{{.Alias}} "flomation.app/automate/executor/actions/{{.Key}}"
{{- end}}
)

var Actions = map[string]core.Action{
{{- range .Entries}}
	"{{.Key}}": {{.Alias}}.Execute,
{{- end}}
}
`))

type actionsGoEntry struct {
	Key   string
	Alias string
}

type actionsGoData struct {
	Imports  []actionsGoEntry
	Entries  []actionsGoEntry
}

func generateActionsGo(manifest map[string]ManifestEntry, outputPath string) {
	var entries []actionsGoEntry

	for key, me := range manifest {
		if !me.HasExecute {
			continue
		}
		alias := strings.ReplaceAll(key, "/", "_")
		entries = append(entries, actionsGoEntry{Key: key, Alias: alias})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	data := actionsGoData{
		Imports: entries,
		Entries: entries,
	}

	var buf bytes.Buffer
	if err := actionsGoTemplate.Execute(&buf, data); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to execute actions.go template")
		return
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"raw":   buf.String(),
		}).Error("unable to format generated actions.go")
		return
	}

	if err := os.WriteFile(outputPath, formatted, 0644); err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"output": outputPath,
		}).Error("unable to write generated actions.go")
	}

	log.WithFields(log.Fields{
		"output": outputPath,
		"count":  len(entries),
	}).Info("generated actions.go")
}

func main() {
	output := flag.String("path", "actions-manifest.json", "Output path for manifest file")
	actionsGoPath := flag.String("actions-go-path", "actions/actions_generated.go", "Output path for generated actions.go")

	flag.Parse()
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

	// Collect category metadata and apply to child actions
	categories := make(map[string]*CategoryMeta)
	for key, me := range manifest {
		if me.HasCategory && me.Category != nil {
			categories[key] = me.Category
		}
	}

	// Apply category metadata to actions and remove category-only entries
	for key, me := range manifest {
		if me.HasCategory && !me.HasExecute {
			delete(manifest, key)
			continue
		}
		// Find the matching category (top-level) and sub-category by checking path prefixes
		topPrefix := ""
		subPrefix := ""
		for catKey := range categories {
			if strings.HasPrefix(key, catKey+"/") || key == catKey {
				segments := strings.Count(catKey, "/")
				if segments == 0 && len(catKey) > len(topPrefix) {
					topPrefix = catKey
				} else if segments > 0 && len(catKey) > len(subPrefix) {
					subPrefix = catKey
				}
			}
		}
		if topPrefix != "" && me.Category == nil {
			me.Category = categories[topPrefix]
			manifest[key] = me
		}
		// Apply sub-category for actions with 3+ path segments
		if subPrefix != "" {
			me.SubCategory = categories[subPrefix]
			manifest[key] = me
		} else {
			// Auto-generate sub-category from path for 3+ segment action IDs without explicit metadata
			parts := strings.Split(key, "/")
			if len(parts) >= 3 {
				subDir := parts[1]
				me.SubCategory = &CategoryMeta{
					Name:        strings.ToUpper(subDir[:1]) + subDir[1:],
					Icon:        "",
					Description: "",
				}
				manifest[key] = me
			}
		}
	}

	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to marshal json manifest")
	}

	if err := os.WriteFile(*output, b, 0600); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to write manifest file")
	}

	generateActionsGo(manifest, *actionsGoPath)
}
