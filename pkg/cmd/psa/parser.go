package psa

import (
	"regexp"
	"strings"
)

const (
	headerString = "existing pods in namespace"
	// submatch in contrast to the whole match.
	submatch = 1
	// firstMatch is the first match.
	firstMatch = 0
	// secondMatch is the second match.
	secondMatch = 1
)

// wordsInParenthesis parses words from parenthesis.
var wordsInParenthesis = regexp.MustCompile(`"([^"]+)"`)

// parseWarnings parses the warnings that are returned by the API request and
// creates a PodSecurityViolation out of it, that contains the namespace, level,
// pod name and violations.
// We expect to have
//
// Example Warning Slice:
// [0] existing pods in namespace "p0t-sekurity-namespace" violate the new PodSecurity enforce level "restricted:latest"
// [1] p0t-sekurity-pod: allowPrivilegeEscalation != false, unrestricted capabilities, runAsNonRoot != true, seccompProfile
// OR
// [1] p0t-sekurity-pod (and 1 other pod): allowPrivilegeEscalation != false, unrestricted capabilities, runAsNonRoot != true, seccompProfile
// OR
// [1] p0t-sekurity-pod (and 10 other pods): allowPrivilegeEscalation != false, unrestricted capabilities, runAsNonRoot != true, seccompProfile
func parseWarnings(warnings []string) *PodSecurityViolation {
	if len(warnings) == 0 {
		return nil
	}

	var psv PodSecurityViolation

	// There should be exactly 2 warnings, but lets not rely on the order nor it to stay that way.
	for _, warning := range warnings {
		// Namespace Warning Message
		if strings.HasPrefix(warning, headerString) {
			// The text should look like "existing pods in namespace "my-namespace" violate the new PodSecurity enforce level "mylevel:v1.2.3"
			titleMatches := wordsInParenthesis.FindAllStringSubmatch(warning, 2)
			psv.Namespace = titleMatches[firstMatch][submatch]
			psv.Level = titleMatches[secondMatch][submatch]
			continue
		}

		// The text should look like this: {pod name}{(optional) more pods hint}: {policy warning A}, {policy warning B}, ...
		textSplit := strings.Split(warning, ": ")
		// Get rid of the potential " (and x other pods)" hint.
		psv.PodName = strings.Split(textSplit[firstMatch], " ")[firstMatch]
		psv.Violations = strings.Split(textSplit[secondMatch], ", ")
	}

	return &psv
}
