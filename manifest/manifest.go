// Package manifest implements the TAC Language manifest system.
//
// The manifest tracks a flow's declared inputs, outputs, used skills,
// agent dependencies, and trust type flows. It serves as a structured
// descriptor that can be serialized to JSON alongside the compiled flow.
package manifest

import (
	"sort"

	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/types"
)

// Manifest describes the metadata and dependencies of a TAC flow.
type Manifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Inputs      []InputDecl       `json:"inputs,omitempty"`
	Outputs     []OutputDecl      `json:"outputs,omitempty"`
	Skills      []SkillUsage      `json:"skills,omitempty"`
	Agents      []AgentRef        `json:"agents,omitempty"`
	TypeFlows   []TypeFlow        `json:"type_flows,omitempty"`
	NodeCount   int               `json:"node_count"`
	EdgeCount   int               `json:"edge_count"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// InputDecl describes a flow input declaration.
type InputDecl struct {
	Name      string         `json:"name"`
	TrustType types.TrustType `json:"trust_type"`
	Required  bool           `json:"required"`
}

// OutputDecl describes a flow output.
type OutputDecl struct {
	Name      string         `json:"name"`
	NodeRef   string         `json:"node_ref"`
	TrustType types.TrustType `json:"trust_type"`
}

// SkillUsage records which skill a node uses.
type SkillUsage struct {
	Skill string `json:"skill"`
	Node  string `json:"node"`
}

// AgentRef records an agent declaration.
type AgentRef struct {
	Name  string `json:"name"`
	Model string `json:"model,omitempty"`
}

// TypeFlow records trust type transformations between nodes.
type TypeFlow struct {
	FromNode string         `json:"from_node"`
	ToNode   string         `json:"to_node"`
	FromType types.TrustType `json:"from_type"`
	ToType   types.TrustType `json:"to_type"`
}

// ExtractManifest builds a Manifest from a flow AST node.
func ExtractManifest(flow *ast.Node) *Manifest {
	m := &Manifest{
		Name:        flow.Value,
		Version:     "1.0",
		Annotations: make(map[string]string),
	}

	nodes := ast.CollectNodes(flow)
	edges := ast.CollectEdges(flow)
	m.NodeCount = len(nodes)
	m.EdgeCount = len(edges)

	// Extract inputs from flow children
	for _, child := range flow.Children {
		if child.Type == ast.NodeInput && len(child.Children) >= 1 {
			input := InputDecl{
				Name:     child.Children[0].Value,
				Required: true,
			}
			input.TrustType = types.Untrusted
			if len(child.Children) >= 2 && types.IsValidTrustType(child.Children[1].Value) {
				input.TrustType = types.TrustType(child.Children[1].Value)
			}
			m.Inputs = append(m.Inputs, input)
		}
	}

	// Extract agent declarations
	for _, child := range flow.Children {
		if child.Type == ast.NodeAgentDecl {
			ag := AgentRef{}
			if len(child.Children) >= 1 {
				ag.Name = child.Children[0].Value
			}
			if child.Attrs != nil {
				if model, ok := child.Attrs["model"]; ok && model != nil {
					ag.Model = model.Value
				}
			}
			m.Agents = append(m.Agents, ag)
		}
	}

	// Extract skills from nodes
	nodeSkills := make(map[string]string) // node -> skill
	for _, node := range nodes {
		nodeName := ast.NodeName(node)
		for _, child := range node.Children {
			if child.Type == ast.NodeSkillCall {
				skillName := child.Value
				nodeSkills[nodeName] = skillName
				m.Skills = append(m.Skills, SkillUsage{
					Skill: skillName,
					Node:  nodeName,
				})
			}
		}
		// Nested blocks
		for _, sub := range node.Nodes {
			if sub.Type == ast.NodeSkillCall {
				skillName := sub.Value
				nodeSkills[nodeName] = skillName
				m.Skills = append(m.Skills, SkillUsage{
					Skill: skillName,
					Node:  nodeName,
				})
			}
		}
	}

	// Sort skills for deterministic output
	sort.Slice(m.Skills, func(i, j int) bool {
		if m.Skills[i].Node != m.Skills[j].Node {
			return m.Skills[i].Node < m.Skills[j].Node
		}
		return m.Skills[i].Skill < m.Skills[j].Skill
	})

	// Sort agents
	sort.Slice(m.Agents, func(i, j int) bool {
		return m.Agents[i].Name < m.Agents[j].Name
	})

	return m
}

// SkillsByNode returns a map of node name to skill name.
func (m *Manifest) SkillsByNode() map[string]string {
	result := make(map[string]string, len(m.Skills))
	for _, su := range m.Skills {
		result[su.Node] = su.Skill
	}
	return result
}

// UsedSkills returns a deduplicated list of skill names used.
func (m *Manifest) UsedSkills() []string {
	seen := make(map[string]bool)
	var result []string
	for _, su := range m.Skills {
		if !seen[su.Skill] {
			seen[su.Skill] = true
			result = append(result, su.Skill)
		}
	}
	sort.Strings(result)
	return result
}

// MergeManifests combines multiple flow manifests into a single program manifest.
func MergeManifests(manifests []*Manifest) *Manifest {
	merged := &Manifest{
		Name:    "program",
		Version: "1.0",
	}

	skillSeen := make(map[string]bool)
	agentSeen := make(map[string]bool)

	for _, m := range manifests {
		merged.NodeCount += m.NodeCount
		merged.EdgeCount += m.EdgeCount
		merged.Inputs = append(merged.Inputs, m.Inputs...)
		merged.TypeFlows = append(merged.TypeFlows, m.TypeFlows...)

		for _, su := range m.Skills {
			key := su.Skill + ":" + su.Node
			if !skillSeen[key] {
				skillSeen[key] = true
				merged.Skills = append(merged.Skills, su)
			}
		}
		for _, ag := range m.Agents {
			if !agentSeen[ag.Name] {
				agentSeen[ag.Name] = true
				merged.Agents = append(merged.Agents, ag)
			}
		}
	}

	return merged
}
