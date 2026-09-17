package config

import (
	"fmt"
	"strings"
)

// TableRef identifies a table using its keyspace and table name.
type TableRef struct {
	Keyspace string `yaml:"keyspace"`
	Table    string `yaml:"table"`
}

// TableMapping describes a one-way synchronization from a source table to a target table
// that shares the same schema. Every write to the source table is forwarded to the target
// table with only the table name (and optionally keyspace) changed.
type TableMapping struct {
	Name    string   `yaml:"name"`
	Enabled bool     `yaml:"enabled"`
	Source  TableRef `yaml:"source"`
	Target  TableRef `yaml:"target"`
}

// ValidateTableMappings validates mapping structure without requiring a cluster connection.
func ValidateTableMappings(mappings []TableMapping) error {
	names := make(map[string]struct{}, len(mappings))
	sources := make(map[string]struct{}, len(mappings))

	for i, mapping := range mappings {
		if err := validateTableMapping(mapping); err != nil {
			return fmt.Errorf("table_mappings[%d]: %w", i, err)
		}

		name := strings.ToLower(mapping.Name)
		if _, exists := names[name]; exists {
			return fmt.Errorf("table_mappings[%d]: duplicate mapping name %q", i, mapping.Name)
		}
		names[name] = struct{}{}

		if !mapping.Enabled {
			continue
		}
		source := tableRefKey(mapping.Source)
		if _, exists := sources[source]; exists {
			return fmt.Errorf("table_mappings[%d]: source %s.%s already has an enabled mapping", i, mapping.Source.Keyspace, mapping.Source.Table)
		}
		sources[source] = struct{}{}
	}

	return nil
}

func validateTableMapping(mapping TableMapping) error {
	if strings.TrimSpace(mapping.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if err := validateTableRef("source", mapping.Source); err != nil {
		return err
	}
	if err := validateTableRef("target", mapping.Target); err != nil {
		return err
	}
	if tableRefKey(mapping.Source) == tableRefKey(mapping.Target) {
		return fmt.Errorf("source and target tables must be different")
	}
	return nil
}

func validateTableRef(label string, ref TableRef) error {
	if strings.TrimSpace(ref.Keyspace) == "" {
		return fmt.Errorf("%s keyspace is required", label)
	}
	if strings.TrimSpace(ref.Table) == "" {
		return fmt.Errorf("%s table is required", label)
	}
	return nil
}

func tableRefKey(ref TableRef) string {
	return strings.ToLower(ref.Keyspace) + "." + strings.ToLower(ref.Table)
}
