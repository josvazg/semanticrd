package semanticrd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	yaml "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Path []string

func NewPath(path string) Path {
	parts := strings.Split(path, ".")
	if len(parts) > 0 && parts[0] == "" {
		return parts[1:]
	}
	return parts
}

type OverrideType string

const (
	// Identified is the namspace for indentifier
	Identifier = "identifier"

	// References is the namespace for refenreces
	References = "references"
)

const (
	IDType     OverrideType = "ID"
	RefType    OverrideType = "REFERENCE"
	SecretType OverrideType = "SECRET"
)

type SemanticRules struct {
	Group     string               `yaml:"group"`
	Versions  []string             `yaml:"versions"`
	Globals   []Mapping            `yaml:"globals,omitempty"`
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

func (m Mapping) Apply(obj map[string]interface{}, version string) ([]map[string]interface{}, error) {
	switch m.Type {
	case IDType:
		return m.Id.apply(obj, version)
	case RefType:
		return m.Ref.apply(obj, version)
	case SecretType:
		return m.Secret.apply(obj, version)
	default:
		panic(fmt.Errorf("unsupported type %v", m.Type))
	}
}

type Id struct {
	Path string `yaml:"path"`
}

func (id Id) apply(obj map[string]interface{}, version string) ([]map[string]interface{}, error) {
	src := append([]string{"spec", version}, NewPath(id.Path)...)
	dst := append([]string{"spec", version, Identifier}, NewPath(id.Path)...)
	err := move(obj, obj, src, dst)
	if err != nil {
		return nil, err
	}
	return nil, err
}

type Ref struct {
	Path       string `yaml:"path"`
	ParentKind string `yaml:"parentKind"`
}

func (r Ref) apply(obj map[string]interface{}, version string) ([]map[string]interface{}, error) {
	src := append([]string{"spec", version}, NewPath(r.Path)...)
	dst := append([]string{"spec", version, References}, NewPath(r.Path)...)
	err := move(obj, obj, src, dst)
	if err != nil {
		return nil, err
	}
	return nil, err
}

type Secret struct {
	Name   string  `yaml:"name"`
	Fields []Field `yaml:"fields,omitempty"`
}

func (s Secret) apply(obj map[string]interface{}, version string) ([]map[string]interface{}, error) {
	objName, found, err := unstructured.NestedFieldNoCopy(obj, "metadata", "name")
	if !found {
		return nil, fmt.Errorf("not found metadata.name for object %v", obj)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata.name: %w", err)
	}
	secretName := fmt.Sprintf("%s-secret", objName)
	secretCRD := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name": secretName,
		},
	}
	for _, field := range s.Fields {
		src := append([]string{"spec", version}, NewPath(field.Path)...)
		_, found, _ := unstructured.NestedFieldNoCopy(obj, src...)
		if !found {
			return nil, nil
		}
	}
	for _, field := range s.Fields {
		src := append([]string{"spec", version}, NewPath(field.Path)...)
		dst := append([]string{"data"}, NewPath(field.Path)...)
		err := move(obj, secretCRD, src, dst)
		if err != nil {
			return nil, fmt.Errorf("failed to move secret field %s to %s: %w",
				strings.Join(src, "."), strings.Join(dst, "."), err)
		}
	}
	refDst := append([]string{"spec", version}, NewPath(s.Name)...)
	if err := unstructured.SetNestedField(obj, secretName, refDst...); err != nil {
		return nil, fmt.Errorf("failed to set secret ref %s to %s: %w", s.Name, secretName, err)
	}
	return []map[string]interface{}{secretCRD}, nil
}

type Field struct {
	Path   string  `yaml:"path"`
	Rename *string `yaml:"rename,omitempty"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

func Apply(out io.Writer, in, semantics io.Reader) error {
	semDec := yaml.NewDecoder(semantics)
	var rules SemanticRules
	if err := semDec.Decode(&rules); err != nil {
		return fmt.Errorf("failed to parse semantic rules: %w", err)
	}

	inputDec := yaml.NewDecoder(in)
	for i := 0; ; i++ {
		var crd map[string]interface{}
		if err := inputDec.Decode(&crd); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to parse input CRDs: %w", err)
		}
		outputEnc := yaml.NewEncoder(out)

		kind := (crd["kind"]).(string)
		mappings := mappingsFor(rules, kind)
		outCRDs, err := apply(crd, rules.Versions, mappings)
		if err != nil {
			return fmt.Errorf("failed to apply rules: %w", err)
		}
		if i != 0 {
			out.Write(([]byte)("---\n"))
		}
		for _, outCRD := range outCRDs {
			if err := outputEnc.Encode(outCRD); err != nil {
				return fmt.Errorf("failed to decode output CRD: %w", err)
			}
		}
	}
	return nil
}

func mappingsFor(rules SemanticRules, kind string) map[string]Mapping {
	maps := map[string]Mapping{}
	for _, mapping := range append(rules.Globals, rules.Overrides[kind]...) {
		maps[mapping.UniqueId()] = mapping
	}
	return maps
}

func apply(crd map[string]interface{}, versions []string, mappings map[string]Mapping) ([]map[string]interface{}, error) {
	allExtras := []map[string]interface{}{}
	for _, version := range versions {
		_, found, _ := unstructured.NestedFieldNoCopy(crd, "spec", version)
		if !found {
			continue
		}
		for _, mapping := range mappings {
			extra, err := mapping.Apply(crd, version)
			if err != nil {
				return nil, fmt.Errorf("failed to apply mapping %v: %w", mapping.Type, err)
			}
			allExtras = append(allExtras, extra...)
		}
	}
	return append(allExtras, []map[string]interface{}{crd}...), nil
}

func move(srcObj, dstObj map[string]interface{}, src []string, dst []string) error {
	value, found, err := unstructured.NestedFieldNoCopy(srcObj, src...)
	if !found {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read field %s: %w", strings.Join(src, "."), err)
	}
	err = unstructured.SetNestedField(dstObj, sanitize(value), dst...)
	if err != nil {
		return fmt.Errorf("failed to set field %s to %v: %w", strings.Join(dst, "."), value, err)
	}
	unstructured.RemoveNestedField(srcObj, src...)
	return nil
}

func sanitize(value any) any {
	switch v := (value).(type) {
	case int:
		return int64(v)
	}
	return value
}
