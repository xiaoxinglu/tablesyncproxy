package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validTableMapping() TableMapping {
	return TableMapping{
		Name:    "users-sync",
		Enabled: true,
		Source:  TableRef{Keyspace: "test", Table: "users_by_id"},
		Target:  TableRef{Keyspace: "test", Table: "users_by_email"},
	}
}

func TestValidateTableMappingsAcceptsValidMapping(t *testing.T) {
	require.NoError(t, ValidateTableMappings([]TableMapping{validTableMapping()}))
}

func TestValidateTableMappingsAcceptsDisabledMapping(t *testing.T) {
	m := validTableMapping()
	m.Enabled = false
	require.NoError(t, ValidateTableMappings([]TableMapping{m}))
}

func TestValidateTableMappingsRejectsEmptyName(t *testing.T) {
	m := validTableMapping()
	m.Name = "  "
	err := ValidateTableMappings([]TableMapping{m})
	require.EqualError(t, err, "table_mappings[0]: name is required")
}

func TestValidateTableMappingsRejectsEmptySourceKeyspace(t *testing.T) {
	m := validTableMapping()
	m.Source.Keyspace = ""
	err := ValidateTableMappings([]TableMapping{m})
	require.EqualError(t, err, "table_mappings[0]: source keyspace is required")
}

func TestValidateTableMappingsRejectsEmptyTargetTable(t *testing.T) {
	m := validTableMapping()
	m.Target.Table = ""
	err := ValidateTableMappings([]TableMapping{m})
	require.EqualError(t, err, "table_mappings[0]: target table is required")
}

func TestValidateTableMappingsRejectsSameSourceAndTarget(t *testing.T) {
	m := validTableMapping()
	m.Target = m.Source
	err := ValidateTableMappings([]TableMapping{m})
	require.EqualError(t, err, "table_mappings[0]: source and target tables must be different")
}

func TestValidateTableMappingsRejectsDuplicateName(t *testing.T) {
	first := validTableMapping()
	second := validTableMapping()
	second.Source.Table = "other_table"
	second.Target.Table = "other_target"

	err := ValidateTableMappings([]TableMapping{first, second})
	require.EqualError(t, err, `table_mappings[1]: duplicate mapping name "users-sync"`)
}

func TestValidateTableMappingsRejectsDuplicateEnabledSource(t *testing.T) {
	first := validTableMapping()
	second := validTableMapping()
	second.Name = "second-mapping"
	second.Target.Table = "yet_another_target"

	err := ValidateTableMappings([]TableMapping{first, second})
	require.EqualError(t, err, "table_mappings[1]: source test.users_by_id already has an enabled mapping")
}

func TestValidateTableMappingsAllowsDuplicateSourceWhenOneDisabled(t *testing.T) {
	first := validTableMapping()
	first.Enabled = false
	second := validTableMapping()
	second.Name = "second-mapping"
	second.Target.Table = "yet_another_target"

	require.NoError(t, ValidateTableMappings([]TableMapping{first, second}))
}
