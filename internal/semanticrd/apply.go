package semanticrd

import (
	"fmt"
	"io"
	"log"

	yaml "gopkg.in/yaml.v3"
)

type XPath string

type OverrideType string

const (
	IDType      OverrideType = "ID"
	RefType     OverrideType = "REFERENCE"
	SecretType OverrideType = "SECRET"
)

type SemanticRules struct {
	Group     string              `yaml:"group"`
	Versions  []string            `yaml:"versions"`
	Globals   []Mapping           `yaml:"globals,omitempty"`
	Overrides map[string][]Mapping `yaml:"overrides"`
}

type Mapping struct {
	Type   OverrideType `yaml:"type"`
	Id     *Id          `yaml:"id,omitempty"`
	Ref    *Ref         `yaml:"ref,omitempty"`
	Secret *Secret      `yaml:"secret,omitempty"`
}

func (m Mapping) UniqueId() string {
	switch m.Type {
	case IDType:
		// TODO: Check Id pointer
		return fmt.Sprintf("%s/%s", m.Type, m.Id.Path)
	case RefType:
		// TODO: Check Ref pointer
		return fmt.Sprintf("%s/%s", m.Type, m.Ref.Path)
	case SecretType:
		// TODO: Check Secret pointer
		return fmt.Sprintf("%s/%s", m.Type, m.Secret.Name)
	default:
		panic(fmt.Errorf("unsupported type %v", m.Type))
	}
}

type Id struct {
	Path XPath `yaml:"path"`
}

type Ref struct {
	Path       XPath  `yaml:"path"`
	ParentKind string `yaml:"parentKind"`
}

type Secret struct {
	Name   string  `yaml:"name"`
	Fields []Field `yaml:"fields,omitempty"`
}

type Field struct {
	Path   XPath   `yaml:"path"`
	Rename *string `yaml:"rename,omitempty"`
}

type CRDPlaceHolder struct {
	Kind       string         `yaml:"kind"`
	ApiVersion string         `yaml:"apiVersion"`
	Spec       map[string]any `yaml:"spec"`
}

func Apply(out io.Writer, in, semantics io.Reader) error {
	semDec := yaml.NewDecoder(semantics)
	var rules SemanticRules
	if err := semDec.Decode(&rules); err != nil {
		return fmt.Errorf("failed to parse semantic rules: %w", err)
	}

	inputDec := yaml.NewDecoder(in)
	var crd CRDPlaceHolder
	if err := inputDec.Decode(&crd); err != nil {
		return fmt.Errorf("failed to parse input CRDs: %w", err)
	}

	mappings := mappingsFor(rules, crd.Kind)
	log.Printf("mappings=%v", mappings)
	return nil
}

func mappingsFor(rules SemanticRules, kind string) map[string]Mapping {
	maps := map[string]Mapping{}
	for _, mapping := range append(rules.Globals, rules.Overrides[kind]...) {
		maps[mapping.UniqueId()] = mapping
	}
	return maps
}
