package psa

import (
	"reflect"
	"testing"
)

func TestParseWarnings(t *testing.T) {
	tests := []struct {
		name     string
		warnings []string
		expected *PodSecurityViolation
	}{
		{
			name:     "empty warnings",
			warnings: []string{},
			expected: nil,
		},
		{
			name: "invalid warnings",
			warnings: []string{
				`this is some warning that is not related to pod security admission`,
				`this is some more detail on that unknown warning`,
			},
			expected: nil,
		},
		{
			name: "valid warnings",
			warnings: []string{
				`existing pods in namespace "p0t-sekurity-namespace" violate the new PodSecurity enforce level "restricted:latest"`,
				`p0t-sekurity-pod (and 11 other pods): allowPrivilegeEscalation != false, unrestricted capabilities, runAsNonRoot != true, seccompProfile`,
			},
			expected: &PodSecurityViolation{
				Namespace:  "p0t-sekurity-namespace",
				Level:      "restricted:latest",
				PodName:    "p0t-sekurity-pod",
				Violations: []string{"allowPrivilegeEscalation != false", "unrestricted capabilities", "runAsNonRoot != true", "seccompProfile"},
			},
		},
		{
			name: "valid warnings, mixed with invalid warnings",
			warnings: []string{
				`this is some warning that is not related to pod security admission`,
				`this is some more detail on that unknown warning`,
				`existing pods in namespace "p0t-sekurity-namespace" violate the new PodSecurity enforce level "restricted:latest"`,
				`p0t-sekurity-pod (and 11 other pods): forbidden AppArmor profiles, forbidden sysctls, host namespaces, hostPath volumes, hostPort, hostProcess, non-default capabilities, privileged, procMount, restricted volume types, runAsUser=0, seLinuxOptions`,
				`this is some more warning that is not related to pod security admission`,
				`this is some even more detail on that unknown warning`,
			},
			expected: &PodSecurityViolation{
				Namespace:  "p0t-sekurity-namespace",
				Level:      "restricted:latest",
				PodName:    "p0t-sekurity-pod",
				Violations: []string{"forbidden AppArmor profiles", "forbidden sysctls", "host namespaces", "hostPath volumes", "hostPort", "hostProcess", "non-default capabilities", "privileged", "procMount", "restricted volume types", "runAsUser=0", "seLinuxOptions"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := parseWarnings(tc.warnings)
			if tc.expected == nil && actual != nil {
				t.Errorf("expected: %q, actual: %q", tc.expected, actual)
			}

			if tc.expected != nil && !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("expected: %v, actual: %v", tc.expected, actual)
			}
		})
	}
}
