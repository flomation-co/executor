package core

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

const (
	TriggerTypeManual = "manual"
)

var (
	ErrNoStartNode = errors.New("no start node specified")
	ErrInvalidNode = errors.New("invalid node")
)

type Action func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error)

type Edge struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type Connection struct {
	ID    *string     `json:"id"`
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

func (c *Connection) String() string {
	if c.Type != "string" {
		return ""
	}

	v, ok := c.Value.(string)
	if !ok {
		return ""
	}

	return v
}

func (c *Connection) Number() int64 {
	if c.Type != "integer" {
		return 0
	}

	v, ok := c.Value.(int64)
	if !ok {
		v, ok := c.Value.(float64)
		if !ok {
			v, ok := c.Value.(int)
			if !ok {
				v, err := strconv.ParseInt(c.Value.(string), 10, 64)
				if err != nil {
					return 0
				}

				return v
			}

			return int64(v)
		}

		return int64(v)
	}

	return v
}

func (c *Connection) Boolean() bool {
	if c.Type != "boolean" {
		return false
	}

	v, ok := c.Value.(bool)
	if !ok {
		return false
	}

	return v
}

func FindConnection(name string, connections []*Connection) *Connection {
	for _, c := range connections {
		if c.Name == name {
			return c
		}
	}

	return nil
}

type NodeConfig struct {
	ID      string        `json:"id"`
	Name    *string       `json:"name"`
	Type    int64         `json:"type"`
	Plugin  string        `json:"plugin"`
	Inputs  []*Connection `json:"inputs"`
	Outputs []*Connection `json:"outputs"`
}

type NodeResult struct {
	Inputs  []*Connection `json:"inputs"`
	Outputs []*Connection `json:"outputs"`
}

type NodeData struct {
	ID      string     `json:"id"`
	Label   string     `json:"label"`
	Config  NodeConfig `json:"config"`
	Results NodeResult `json:"results"`
}

type Node struct {
	ID   string    `json:"id"`
	Type string    `json:"type"`
	Data *NodeData `json:"data"`
}

type Flow struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`

	nodeResults map[string]map[string]interface{}
	outputs     map[string]interface{}
}

func Load(path *string) (*Flow, error) {
	if path == nil {
		return nil, nil
	}

	b, err := os.ReadFile(*path)
	if err != nil {
		return nil, err
	}

	var f Flow
	if err = json.Unmarshal(b, &f); err != nil {
		return nil, err
	}

	f.nodeResults = make(map[string]map[string]interface{})
	f.outputs = make(map[string]interface{})

	log.WithFields(log.Fields{
		"nodes": len(f.Nodes),
		"edges": len(f.Edges),
	}).Debug("Loaded Flow")

	return &f, nil
}

func (f *Flow) Execute(actions map[string]Action, entry *string, environment *string) (map[string]interface{}, error) {
	var start *Node

	if entry != nil {
		start = f.FindNode(*entry)
	} else {
		for _, n := range f.Nodes {
			if n == nil {
				continue
			}

			if n.Type == TriggerTypeManual {
				start = n
				break
			}
		}
	}

	if start == nil {
		return nil, ErrNoStartNode
	}

	_, err := f.ExecuteNode(actions, start, environment)
	if err != nil {
		return nil, err
	}

	return f.outputs, nil
}

func (f *Flow) ExecuteNode(actions map[string]Action, node *Node, environment *string) (map[string]interface{}, error) {
	var err error

	if node == nil || node.Data == nil {
		return nil, ErrInvalidNode
	}

	if v, exists := f.nodeResults[node.ID]; exists {
		log.WithFields(log.Fields{
			"id": node.ID,
		}).Debug("Node cached, returning")
		return v, nil
	}

	log.WithFields(log.Fields{
		"node": node,
	}).Debug("Processing Node")

	var results map[string]interface{}
	parents := f.FindSource(node.ID)
	for _, p := range parents {
		if p == nil {
			continue
		}

		results, err = f.ExecuteNode(actions, p, environment)
		if err != nil {
			return nil, err
		}

		f.nodeResults[node.ID] = results

		for k, v := range results {
			results[k] = v
		}
	}

	a, exists := actions[node.Type]
	if !exists {
		log.WithFields(log.Fields{
			"type": node.Type,
		}).Debug("unknown node action")
		return nil, ErrInvalidNode
	}

	var configuration []*Connection
	for _, v := range node.Data.Config.Inputs {
		value := v.Value
		if _, exists := results[v.Name]; exists {
			value = results[v.Name]
		}

		// TODO: Variable substitution
		configuration = append(configuration, &Connection{
			ID:    v.ID,
			Name:  v.Name,
			Type:  v.Type,
			Value: value,
		})
	}

	outputs, err := a(f, node, configuration)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Error processing Action")
		//	TODO: Determine what to do in Error scenario
	}

	f.nodeResults[node.ID] = outputs

	log.WithFields(log.Fields{
		"results": outputs,
	}).Debug("Node results")

	combinedResults := make(map[string]interface{})

	children := f.FindTarget(node.ID)
	for _, c := range children {
		childResults, err := f.ExecuteNode(actions, c, environment)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Error processing Child")
		}

		for k, v := range childResults {
			combinedResults[k] = v
		}
	}

	return combinedResults, nil
}

func (f *Flow) FindSource(target string) []*Node {
	results := make([]*Node, 0)

	for _, e := range f.Edges {
		if e == nil {
			continue
		}

		if e.Target == target {
			n := f.FindNode(e.Source)
			if n != nil {
				results = append(results, n)
			}
		}
	}

	return results
}

func (f *Flow) FindTarget(source string) []*Node {
	results := make([]*Node, 0)

	for _, e := range f.Edges {
		if e == nil {
			continue
		}

		if e.Source == source {
			n := f.FindNode(e.Target)
			if n != nil {
				results = append(results, n)
			}
		}
	}

	return results
}

func (f *Flow) FindNode(id string) *Node {
	for _, n := range f.Nodes {
		if n.ID == id {
			return n
		}
	}

	return nil
}

func (f *Flow) SetOutput(name string, value interface{}) {
	if _, exists := f.outputs[name]; exists {
		log.WithFields(log.Fields{
			"value": name,
		}).Warn("overwriting already set output value")
	}

	f.outputs[name] = value
}

func (f *Flow) GetOutput(name string) interface{} {
	return f.outputs[name]
}
