package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/josvazg/semanticrd/internal/semanticrd"
)

func apply(output, input, semantics string) error {
	in, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("failed to open input file %q: %w", input, err)
	}
	defer in.Close()
	
	rules, err := os.Open(semantics)
	if err != nil {
		return fmt.Errorf("failed to open semantics file %q: %w", semantics, err)
	}
	defer rules.Close()

	out, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %w", output, err)
	}
	defer out.Close()
	
	return semanticrd.Apply(out, in, rules)
}

func main() {
	var input, semantics, output string
	flag.StringVar(&input, "input", "crds.yaml", "input YAML to process")
	flag.StringVar(&semantics, "semantics", "semantics.yaml", "semantics YAML to apply")
	flag.StringVar(&output, "output", "crds-processes.yaml", "output YAML to produce")
	err := apply(output, input, semantics)
	if err != nil {
		log.Fatalf("Failed to apply semantics %s on %s: %v", semantics, input, err)
	}
	log.Printf("Semantics applied to %s", output)
}
