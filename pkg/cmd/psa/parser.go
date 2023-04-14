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

var (
	// wordsInParenthesis parses words from parenthesis.
	wordsInParenthesis = regexp.MustCompile(`"([^"]+)"`)
	violations         = map[string]struct{}{
		"allowPrivilegeEscalation != false": empty,
		"forbidden AppArmor profile":        empty,
		"forbidden AppArmor profiles":       empty,
		"forbidden sysctls":                 empty,
		"host namespaces":                   empty,
		"hostPath volumes":                  empty,
		"hostPort":                          empty,
		"hostProcess":                       empty,
		"non-default capabilities":          empty,
		"privileged":                        empty,
		"procMount":                         empty,
		"restricted volume types":           empty,
		"runAsNonRoot != true":              empty,
		"runAsUser=0":                       empty,
		"seccompProfile":                    empty,
		"seLinuxOptions":                    empty,
		"unrestricted capabilities":         empty,
	}
)

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
		regex := regexp.MustCompile(`existing pods in namespace "([^"]+)" violate the new PodSecurity enforce level "([^"]+)"`)
		matches := regex.FindStringSubmatch(warning)

		// Namespace Warning Message
		if len(matches) == 3 {
			// The text should look like "existing pods in namespace "my-namespace" violate the new PodSecurity enforce level "mylevel:v1.2.3"
			psv.Namespace = matches[1]
			psv.Level = matches[2]
			continue
		}

		// The text should look like this: {pod name}{(optional) more pods hint}: {policy warning A}, {policy warning B}, ...
		// Verify that we are handling violations
		textSplit := strings.Split(warning, ": ")
		matchingList := strings.Split(textSplit[secondMatch], ", ")
		matchedCandidates := []string{}
		for _, candidate := range matchingList {
			if _, ok := violations[candidate]; ok {
				matchedCandidates = append(matchedCandidates, candidate)
				continue
			}
		}

		if len(matchedCandidates) == 0 {
			continue
		}

		// Get rid of the potential " (and x other pods)" hint.
		psv.PodName = strings.Split(textSplit[firstMatch], " ")[firstMatch]
		psv.Violations = matchedCandidates
	}

	return &psv
}
