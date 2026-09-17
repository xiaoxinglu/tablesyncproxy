package zdmproxy

import (
	"strings"
	"testing"

	"github.com/datastax/zdm-proxy/proxy/pkg/config"
	"github.com/stretchr/testify/require"
)

// rewriteRegistry builds a TableMappingRegistry wired to test.users_by_id -> test.users_by_email1.
func rewriteRegistry() *TableMappingRegistry {
	return NewTableMappingRegistry([]config.TableMapping{
		{
			Name:    "users-by-email",
			Enabled: true,
			Source:  config.TableRef{Keyspace: "test", Table: "users_by_id"},
			Target:  config.TableRef{Keyspace: "test", Table: "users_by_email1"},
		},
	})
}

// ---------------------------------------------------------------------------
// INSERT
// ---------------------------------------------------------------------------

func TestPlanRewriteInsertQualified(t *testing.T) {
	query := "INSERT INTO test.users_by_id (user_id, email, name) VALUES (?, ?, ?)"
	info := inspectCqlQuery(query, "", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, query, plan.SourceQuery)
	require.Equal(t,
		"INSERT INTO test.users_by_email1 (user_id, email, name) VALUES (?, ?, ?)",
		plan.TargetQuery)
}

func TestPlanRewriteInsertUnqualified(t *testing.T) {
	// The client uses a USE keyspace so the query is unqualified.
	query := "INSERT INTO users_by_id (user_id, email, name) VALUES (?, ?, ?)"
	info := inspectCqlQuery(query, "test", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, "INSERT INTO test.users_by_email1 (user_id, email, name) VALUES (?, ?, ?)", plan.TargetQuery)
}

func TestPlanRewriteInsertWithLiterals(t *testing.T) {
	query := "INSERT INTO test.users_by_id (user_id, email, name) VALUES (12, 'alice@example.com', 'Alice')"
	info := inspectCqlQuery(query, "", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t,
		"INSERT INTO test.users_by_email1 (user_id, email, name) VALUES (12, 'alice@example.com', 'Alice')",
		plan.TargetQuery)
}

// ---------------------------------------------------------------------------
// UPDATE
// ---------------------------------------------------------------------------

func TestPlanRewriteUpdate(t *testing.T) {
	query := "UPDATE test.users_by_id SET name = ? WHERE user_id = ?"
	info := inspectCqlQuery(query, "", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t,
		"UPDATE test.users_by_email1 SET name = ? WHERE user_id = ?",
		plan.TargetQuery)
}

// ---------------------------------------------------------------------------
// DELETE
// ---------------------------------------------------------------------------

func TestPlanRewriteDeleteSimple(t *testing.T) {
	query := "DELETE FROM test.users_by_id WHERE user_id = 122"
	info := inspectCqlQuery(query, "", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, "DELETE FROM test.users_by_email1 WHERE user_id = 122", plan.TargetQuery)
}

func TestPlanRewriteDeleteWithQuotedValue(t *testing.T) {
	query := "DELETE FROM test.users_by_id WHERE user_id = 'abc-123'"
	info := inspectCqlQuery(query, "", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, "DELETE FROM test.users_by_email1 WHERE user_id = 'abc-123'", plan.TargetQuery)
}

// ---------------------------------------------------------------------------
// Skipped statement types (SELECT)
// ---------------------------------------------------------------------------

func TestPlanRewriteIgnoresSelect(t *testing.T) {
	query := "SELECT * FROM test.users_by_id WHERE user_id = 1"
	info := inspectCqlQuery(query, "", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.Nil(t, plan)
}

// ---------------------------------------------------------------------------
// Unmapped table
// ---------------------------------------------------------------------------

func TestPlanRewriteIgnoresUnmappedTable(t *testing.T) {
	query := "INSERT INTO test.other_table (id) VALUES (?)"
	info := inspectCqlQuery(query, "", nil)

	plan, err := rewriteRegistry().PlanRewrite(info)

	require.NoError(t, err)
	require.Nil(t, plan)
}

// ---------------------------------------------------------------------------
// Disabled mapping
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// BATCH (via PlanRewrite on individual child queries)
// ---------------------------------------------------------------------------

func TestPlanRewriteBatchInsert(t *testing.T) {
	// Each child of a BATCH is an individual CQL string — PlanRewrite handles them one at a time.
	queries := []string{
		"INSERT INTO test.users_by_id (user_id, email, name) VALUES (4, 'a@b.com', 'Alice')",
		"INSERT INTO test.users_by_id (user_id, email, name) VALUES (5, 'c@d.com', 'Bob')",
	}
	reg := rewriteRegistry()
	for _, q := range queries {
		info := inspectCqlQuery(q, "", nil)
		plan, err := reg.PlanRewrite(info)
		require.NoError(t, err)
		require.NotNil(t, plan)
		require.Contains(t, plan.TargetQuery, "users_by_email1")
		require.NotContains(t, plan.TargetQuery, "users_by_id")
	}
}

// ---------------------------------------------------------------------------
// Text BATCH (cqlsh-style BEGIN BATCH ... APPLY BATCH via OpCodeQuery)
// replaceAllWordCI must replace every occurrence in the full batch string.
// ---------------------------------------------------------------------------

func TestReplaceAllWordCIReplacesMultipleOccurrences(t *testing.T) {
	batch := "BEGIN BATCH " +
		"INSERT INTO test.users_by_id (user_id, email) VALUES (4, 'a@b.com'); " +
		"INSERT INTO test.users_by_id (user_id, email) VALUES (5, 'c@d.com'); " +
		"APPLY BATCH"

	result, n := replaceAllWordCI(batch, "test.users_by_id", "test.users_by_email1")

	require.Equal(t, 2, n)
	require.NotContains(t, result, "users_by_id")
	require.Equal(t, 2, strings.Count(result, "users_by_email1"))
}

func TestReplaceAllWordCIWordBoundary(t *testing.T) {
	// "users_by_id" must not match inside "users_by_id_archive"
	s := "INSERT INTO test.users_by_id_archive (id) VALUES (?)"
	_, n := replaceAllWordCI(s, "users_by_id", "users_by_email1")
	require.Equal(t, 0, n)
}

func TestPlanRewriteIgnoresDisabledMapping(t *testing.T) {
	registry := NewTableMappingRegistry([]config.TableMapping{
		{
			Name:    "disabled-mapping",
			Enabled: false,
			Source:  config.TableRef{Keyspace: "test", Table: "users_by_id"},
			Target:  config.TableRef{Keyspace: "test", Table: "users_by_email1"},
		},
	})
	query := "INSERT INTO test.users_by_id (user_id) VALUES (?)"
	info := inspectCqlQuery(query, "", nil)

	plan, err := registry.PlanRewrite(info)

	require.NoError(t, err)
	require.Nil(t, plan)
}
