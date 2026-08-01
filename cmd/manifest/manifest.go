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

	Category       *CategoryMeta `json:"category,omitempty"`
	SubCategory    *CategoryMeta `json:"sub_category,omitempty"`
	SubSubCategory *CategoryMeta `json:"sub_sub_category,omitempty"`

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
			s, ok := stringValue(kv.Value)
			if !ok {
				continue
			}
			switch key.Name {
			case "Name":
				opt.Name = s
			case "Value":
				opt.Value = s
			case "Group":
				opt.Group = s
			}
		}
		options = append(options, opt)
	}
	return options
}

// stringValue resolves a Go expression that the AST walker expects to be a
// string constant.
//
// It exists because "a" + "b" is not an *ast.BasicLit — it is an
// *ast.BinaryExpr whose operands are. Every site here used to type-assert
// straight to *ast.BasicLit and `continue` when that failed, so a concatenated
// constant was not a parse error, it was a SILENT empty value: the action kept
// building, kept passing tests, and shipped with a blank field in the palette.
// That is exactly what happened to slack/rich_message, whose Description was
// split across lines with `+` and had therefore been empty in the manifest for
// its entire life.
//
// Concatenation is unavoidable in practice — it is how you wrap a long
// description at a sane line length — so the generator has to understand it
// rather than the actions having to avoid it.
//
// The second return reports whether this really was a resolvable string
// constant, so callers can tell "" (a genuinely empty literal) apart from "I
// could not read this", and report the latter instead of swallowing it.
func stringValue(expr ast.Expr) (string, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", false
		}
		return s, true

	case *ast.BinaryExpr:
		// Only `+` concatenates; anything else is not a string constant we can
		// fold (and no other operator is valid on strings anyway).
		if v.Op != token.ADD {
			return "", false
		}
		lhs, ok := stringValue(v.X)
		if !ok {
			return "", false
		}
		rhs, ok := stringValue(v.Y)
		if !ok {
			return "", false
		}
		return lhs + rhs, true

	case *ast.ParenExpr:
		return stringValue(v.X)

	default:
		return "", false
	}
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
			if s, ok := stringValue(kv.Value); ok {
				vw.Field = s
			}
		case "Values":
			if cl, ok := kv.Value.(*ast.CompositeLit); ok {
				for _, elt := range cl.Elts {
					if s, ok := stringValue(elt); ok {
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

					if len(v.Values) == 0 {
						continue
					}
					// A string constant (possibly concatenated) covers every
					// metadata field except Type, which may also be written as a
					// bare int literal — so keep the raw literal too rather than
					// silently dropping that form.
					strVal, isString := stringValue(v.Values[0])
					lit, isLit := v.Values[0].(*ast.BasicLit)
					if !isString && !isLit {

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
									case "ActionTypeAwait":
										me.Type = core.ActionTypeAwait
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
								case *ast.BasicLit, *ast.BinaryExpr, *ast.ParenExpr:
									value, _ = stringValue(v)
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
									case "ConnectionTypeDateTime":
										value = "datetime"
									case "ConnectionTypeMultiSelect":
										value = "multi_select"
									case "ConnectionTypeRows":
										value = "rows"
									case "ConnectionTypeCredential":
										value = "credential"
									case "ConnectionTypeSecret":
										value = "secret"
									case "ConnectionTypeCode":
										value = "code"
									case "ConnectionTypeMoney":
										value = "money"
									case "ConnectionTypeFieldSourceMap":
										value = "field_source_map"
									case "ConnectionTypeComboBox":
										value = "combobox"
									case "ConnectionTypeFile":
										value = "file"
									case "ConnectionTypeColour":
										value = "colour"
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
								case "FromCredentialMeta":
									// Plain string literal only — same constraint as every
									// other field here. A concatenated expression is a
									// BinaryExpr, not a BasicLit, and would silently blank
									// this, which would turn the auto-fill off without any
									// signal. Guarded by the generator's own test.
									c.FromCredentialMeta = value
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
							me.Author = strVal
							meUpdated = true
						case "Organisation":
							me.Organisation = strVal
							meUpdated = true
						case "Name":
							me.Name = strVal
							meUpdated = true
						case "Description":
							me.Description = strVal
							meUpdated = true
						case "Website":
							me.Website = strVal
							meUpdated = true
						case "Icon":
							me.Icon = strVal
							meUpdated = true
						case "Date":
							me.Date = strVal
							meUpdated = true
						case "Type":
							if isLit {
								me.Type, _ = strconv.ParseInt(lit.Value, 10, 64)
								meUpdated = true
							}
						case "CategoryName":
							if me.Category == nil {
								me.Category = &CategoryMeta{}
							}
							me.Category.Name = strVal
							me.HasCategory = true
						case "CategoryIcon":
							if me.Category == nil {
								me.Category = &CategoryMeta{}
							}
							me.Category.Icon = strVal
							me.HasCategory = true
						case "CategoryDescription":
							if me.Category == nil {
								me.Category = &CategoryMeta{}
							}
							me.Category.Description = strVal
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
	Imports []actionsGoEntry
	Entries []actionsGoEntry
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
		// Find the matching category by checking path prefixes at each depth. A
		// category.go with N path segments (N slashes) sets the level-N grouping:
		//   0 slashes → top Category      (e.g. "crm")
		//   1 slash   → Sub-Category      (e.g. "crm/apollo")
		//   2 slashes → Sub-Sub-Category  (e.g. "crm/apollo/enrichment")
		// The longest matching prefix at each level wins.
		topPrefix := ""
		subPrefix := ""
		subSubPrefix := ""
		for catKey := range categories {
			if strings.HasPrefix(key, catKey+"/") || key == catKey {
				switch strings.Count(catKey, "/") {
				case 0:
					if len(catKey) > len(topPrefix) {
						topPrefix = catKey
					}
				case 1:
					if len(catKey) > len(subPrefix) {
						subPrefix = catKey
					}
				case 2:
					if len(catKey) > len(subSubPrefix) {
						subSubPrefix = catKey
					}
				}
			}
		}
		parts := strings.Split(key, "/")
		if topPrefix != "" && me.Category == nil {
			me.Category = categories[topPrefix]
			manifest[key] = me
		}
		// Sub-category: explicit category.go, else auto-generate from parts[1]
		// for actions with 3+ path segments.
		if subPrefix != "" {
			me.SubCategory = categories[subPrefix]
			manifest[key] = me
		} else if len(parts) >= 3 {
			subDir := parts[1]
			me.SubCategory = &CategoryMeta{Name: strings.ToUpper(subDir[:1]) + subDir[1:]}
			manifest[key] = me
		}
		// Sub-sub-category: explicit category.go, else auto-generate from parts[2]
		// for actions with 4+ path segments.
		if subSubPrefix != "" {
			me.SubSubCategory = categories[subSubPrefix]
			manifest[key] = me
		} else if len(parts) >= 4 {
			subSubDir := parts[2]
			me.SubSubCategory = &CategoryMeta{Name: strings.ToUpper(subSubDir[:1]) + subSubDir[1:]}
			manifest[key] = me
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
