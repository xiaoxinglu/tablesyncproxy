package zdmproxy

import (
	"fmt"
	"strings"

	"github.com/datastax/zdm-proxy/proxy/pkg/config"
	log "github.com/sirupsen/logrus"
)

// TableMappingRegistry provides case-insensitive lookup of enabled source mappings.
type TableMappingRegistry struct {
	bySource map[string]config.TableMapping
}

func NewTableMappingRegistry(mappings []config.TableMapping) *TableMappingRegistry {
	bySource := make(map[string]config.TableMapping, len(mappings))
	for _, mapping := range mappings {
		if mapping.Enabled {
			key := tableMappingKey(mapping.Source.Keyspace, mapping.Source.Table)
			bySource[key] = mapping
			log.Infof("table-sync: registered mapping %q: %s.%s -> %s.%s",
				mapping.Name,
				mapping.Source.Keyspace, mapping.Source.Table,
				mapping.Target.Keyspace, mapping.Target.Table)
		}
	}
	return &TableMappingRegistry{bySource: bySource}
}

// Len returns the number of enabled mappings in the registry.
func (r *TableMappingRegistry) Len() int {
	return len(r.bySource)
}

// Find looks up the mapping for the given source keyspace and table.
func (r *TableMappingRegistry) Find(keyspace string, table string) (config.TableMapping, bool) {
	mapping, found := r.bySource[tableMappingKey(keyspace, table)]
	return mapping, found
}

// RewritePlan holds a source query and its rewritten target query.
type RewritePlan struct {
	SourceQuery string
	TargetQuery string
	Mapping     config.TableMapping
}

// PlanRewrite rewrites a source CQL write statement (INSERT, UPDATE, DELETE, BATCH)
// by replacing the source keyspace.table reference with the configured target.
//
// The source and target tables are assumed to have identical schemas, so only the
// table reference is substituted — values, column names, and the WHERE clause are
// forwarded verbatim.
//
// Returns nil, nil when no mapping exists for the source table or the statement
// type is not a write (e.g. SELECT).
func (r *TableMappingRegistry) PlanRewrite(info QueryInfo) (*RewritePlan, error) {
	mapping, found := r.Find(info.getApplicableKeyspace(), info.getTableName())
	if !found {
		return nil, nil
	}

	switch info.getStatementType() {
	case statementTypeInsert, statementTypeUpdate, statementTypeDelete:
		// supported write types — proceed
	default:
		return nil, nil
	}

	targetQuery, err := rewriteTableRef(
		info.getQuery(),
		info.getApplicableKeyspace(), info.getTableName(),
		mapping.Target.Keyspace, mapping.Target.Table,
	)
	if err != nil {
		return nil, fmt.Errorf("mapping %q: %w", mapping.Name, err)
	}

	return &RewritePlan{
		SourceQuery: info.getQuery(),
		TargetQuery: targetQuery,
		Mapping:     mapping,
	}, nil
}

// rewriteTableRef replaces ALL occurrences of the source table reference in the
// CQL query with the target table reference.
//
// It matches both qualified (keyspace.table) and unqualified (table) forms,
// case-insensitively, using word-boundary matching so that a table named "orders"
// does not accidentally match "orders_archive".
//
// Replacing all occurrences is required for text BATCH statements sent via
// OpCodeQuery (e.g. from cqlsh), where the same source table appears once per
// child statement inside BEGIN BATCH ... APPLY BATCH.
func rewriteTableRef(query, srcKs, srcTable, tgtKs, tgtTable string) (string, error) {
	replacement := tgtKs + "." + tgtTable

	// Try qualified form first: keyspace.table
	qualified := srcKs + "." + srcTable
	if rewritten, n := replaceAllWordCI(query, qualified, replacement); n > 0 {
		return rewritten, nil
	}
	// Fall back to unqualified form: bare table name
	if rewritten, n := replaceAllWordCI(query, srcTable, replacement); n > 0 {
		return rewritten, nil
	}
	return "", fmt.Errorf("could not locate table reference %q or %q in query: %s",
		qualified, srcTable, query)
}

// replaceAllWordCI replaces every word-boundary case-insensitive occurrence of
// word in s with replacement. Returns the result and the number of replacements made.
func replaceAllWordCI(s, word, replacement string) (string, int) {
	var result strings.Builder
	count := 0
	start := 0
	for {
		idx := indexWordCI(s[start:], word)
		if idx < 0 {
			result.WriteString(s[start:])
			break
		}
		abs := start + idx
		result.WriteString(s[start:abs])
		result.WriteString(replacement)
		start = abs + len(word)
		count++
	}
	return result.String(), count
}

// indexWordCI returns the byte offset of the first case-insensitive occurrence of word
// in s that is surrounded by non-alphanumeric/non-underscore characters (word boundary).
// Returns -1 if not found.
func indexWordCI(s, word string) int {
	upper := strings.ToUpper(s)
	wordUp := strings.ToUpper(word)
	start := 0
	for {
		idx := strings.Index(upper[start:], wordUp)
		if idx < 0 {
			return -1
		}
		abs := start + idx
		// Check left boundary
		if abs > 0 && isIdentChar(s[abs-1]) {
			start = abs + 1
			continue
		}
		// Check right boundary
		end := abs + len(word)
		if end < len(s) && isIdentChar(s[end]) {
			start = abs + 1
			continue
		}
		return abs
	}
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_'
}

func tableMappingKey(keyspace string, table string) string {
	return strings.ToLower(keyspace) + "." + strings.ToLower(table)
}
