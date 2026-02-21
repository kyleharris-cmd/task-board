package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Policy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy file: %w", err)
	}
	var p Policy
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return Policy{}, fmt.Errorf("parse policy yaml: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Policy{}, fmt.Errorf("validate policy: %w", err)
	}

	return p, nil
}
