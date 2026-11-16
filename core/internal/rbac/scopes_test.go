package rbac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasAnyScope(t *testing.T) {
	t.Parallel()

	require.True(t, HasAnyScope([]string{"a", "b"}, []string{"b", "c"}))
	require.False(t, HasAnyScope([]string{"a"}, []string{"b", "c"}))
	require.False(t, HasAnyScope(nil, []string{"b"}))
	require.True(t, HasAnyScope([]string{"*"}, []string{"anything"}))
}

func TestMissingScope(t *testing.T) {
	t.Parallel()

	require.Equal(t, "b", MissingScope([]string{"a"}, []string{"a", "b"}))
	require.Equal(t, "", MissingScope([]string{"a", "b"}, []string{"a"}))
	require.Equal(t, "", MissingScope([]string{"*"}, []string{"anything"}))
	require.Equal(t, "", MissingScope(nil, nil))
}

func TestSortedScopes(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"a", "b", "c"}, SortedScopes([]string{"c", "a", "b"}))
	require.Empty(t, SortedScopes(nil))
}

func TestScopesForRoleReturnsCopy(t *testing.T) {
	t.Parallel()

	scopes := ScopesForRole(RoleIntern)
	require.NotEmpty(t, scopes)

	scopes[0] = "mutated"

	require.NotEqual(t, "mutated", ScopesForRole(RoleIntern)[0])
}
