package db

import (
	"strings"
	"testing"

	"ai-buddy/core/migrations"

	"github.com/stretchr/testify/require"
)

func TestMigrationFilesAreOrdered(t *testing.T) {
	t.Parallel()

	files, err := migrationFiles()
	require.NoError(t, err)
	require.NotEmpty(t, files)
	require.Equal(t, "0001_init.sql", files[0])

	for i := 1; i < len(files); i++ {
		require.Less(t, files[i-1], files[i])
	}
}

func TestInitialMigrationContainsCoreTables(t *testing.T) {
	t.Parallel()

	body, err := migrations.FS.ReadFile("0001_init.sql")
	require.NoError(t, err)

	sql := string(body)
	for _, table := range []string{
		"teams",
		"users",
		"onboarding_plans",
		"onboarding_stages",
		"onboarding_items",
		"kb_documents",
		"audit_log",
		"events",
	} {
		require.True(t, strings.Contains(sql, "CREATE TABLE IF NOT EXISTS "+table),
			"missing table %s", table)
	}
}
