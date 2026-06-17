package auth

import "testing"

func TestHasRole(t *testing.T) {
	claims := map[string]interface{}{
		"realm_access": map[string]interface{}{
			"roles": []interface{}{"offline_access", "cryo-admin", "uma_authorization"},
		},
		"resource_access": map[string]interface{}{
			"cryo": map[string]interface{}{
				"roles": []interface{}{"viewer"},
			},
		},
	}

	tests := []struct {
		name string
		path string
		role string
		want bool
	}{
		{"nested role present", "realm_access.roles", "cryo-admin", true},
		{"nested role absent", "realm_access.roles", "missing", false},
		{"deep path present", "resource_access.cryo.roles", "viewer", true},
		{"deep path role absent", "resource_access.cryo.roles", "admin", false},
		{"path points to non-array", "realm_access", "cryo-admin", false},
		{"missing intermediate segment", "realm_access.groups.roles", "x", false},
		{"unknown top-level key", "nope.roles", "x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasRole(claims, tt.path, tt.role); got != tt.want {
				t.Fatalf("hasRole(%q, %q) = %v, want %v", tt.path, tt.role, got, tt.want)
			}
		})
	}

	if hasRole(nil, "realm_access.roles", "cryo-admin") {
		t.Fatal("hasRole(nil, ...) should be false")
	}
}
