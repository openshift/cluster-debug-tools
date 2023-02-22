package psa

import (
	"regexp"
	"strings"
)

const headerString = "existing pods in namespace"

var titleRegex = regexp.MustCompile(`"([^"]+)"`)

// parseWarnings parses the warnings that are returned by the API request and
// creates a PodSecurityViolation out of it, that contains the namespace, level,
// pod name and violations.
//
// Example Warning Slice:
// [0] existing pods in namespace "p0t-sekurity" violate the new PodSecurity enforce level "restricted:latest"
// [1] p0t-sekurity: allowPrivilegeEscalation != false, unrestricted capabilities, runAsNonRoot != true, seccompProfile
func parseWarnings(warnings []string) *PodSecurityViolation {
	if len(warnings) == 0 {
		return nil
	}

	var psv PodSecurityViolation

	for _, warning := range warnings {
		// Namespace Warning Message
		if strings.HasPrefix(warning, headerString) {
			// The text should look like "existing pods in namespace "my-namespace" violate the new PodSecurity enforce level "mylevel:v1.2.3"
			titleMatches := titleRegex.FindAllStringSubmatch(warning, -1)
			psv.Namespace = titleMatches[0][1]
			psv.Level = titleMatches[1][1]
			continue
		}

		// The text should look like this: {pod name}: {policy warning A}, {policy warning B}, ...
		textSplit := strings.Split(warning, ": ")
		podName := strings.TrimSpace(textSplit[0])
		violations := strings.Split(textSplit[1], ", ")
		podViolation := PodViolation{
			PodName:    podName,
			Violations: violations,
		}
		psv.PodViolations = append(psv.PodViolations, &podViolation)
	}

	return &psv
}
