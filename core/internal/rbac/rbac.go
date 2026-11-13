package rbac

import (
	"sort"
	"strings"
)

const (
	ScopeAll = "*"

	ScopePlanRead  = "onboarding:plan:read"
	ScopePlanWrite = "onboarding:plan:write"
	ScopeChat      = "chat:use"
	ScopeKBRead    = "kb:read"

	ScopeRequestsRead  = "requests:read"
	ScopeRequestsWrite = "requests:write"
)

const (
	RoleIntern = "intern"
	RoleMentor = "mentor"
	RoleHR     = "hr"
	RoleAdmin  = "admin"
)

var roleScopes = map[string][]string{
	RoleIntern: {ScopePlanRead, ScopePlanWrite, ScopeChat, ScopeKBRead},
	RoleMentor: {ScopePlanRead, ScopePlanWrite, ScopeChat, ScopeKBRead},
	RoleHR:     {ScopePlanRead, ScopeKBRead, ScopeRequestsRead, ScopeRequestsWrite},
	RoleAdmin:  {ScopeAll},
}

func ScopesForRole(role string) []string {
	scopes, ok := roleScopes[strings.ToLower(strings.TrimSpace(role))]
	if !ok {
		return []string{}
	}
	out := make([]string, len(scopes))
	copy(out, scopes)
	return out
}

func HasScope(scopes []string, scope string) bool {
	for _, current := range scopes {
		if current == ScopeAll || current == scope {
			return true
		}
	}
	return false
}

func HasAnyScope(scopes []string, required []string) bool {
	for _, scope := range required {
		if HasScope(scopes, scope) {
			return true
		}
	}
	return false
}

func HasAllScopes(scopes []string, required []string) bool {
	for _, scope := range required {
		if !HasScope(scopes, scope) {
			return false
		}
	}
	return true
}

func MissingScope(scopes []string, required []string) string {
	for _, scope := range required {
		if !HasScope(scopes, scope) {
			return scope
		}
	}
	return ""
}

func SortedScopes(scopes []string) []string {
	out := make([]string, len(scopes))
	copy(out, scopes)
	sort.Strings(out)
	return out
}
