package release_log

import (
	"encoding/json"
	"os/exec"
)

// oc adm release info quay.io/openshift-release-dev/ocp-release:4.6.0-rc.1-x86_64 --commit-urls -o json
// '.references.spec.tags[].annotations."io.openshift.build.source-location"'

type commandOutput struct {
	References struct {
		Spec struct {
			Tags []struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"tags"`
		} `json:"spec"`
	} `json:"references"`
}

func getReleaseImageRepositories(image string) ([]string, error) {
	out, err := exec.Command("oc", "adm", "release", "info", image, "--commit-urls", "-o", "json").CombinedOutput()
	if err != nil {
		return nil, err
	}

	jsonOutput := commandOutput{}
	if err := json.Unmarshal(out, &jsonOutput); err != nil {
		return nil, err
	}
	result := []string{}
	for i := range jsonOutput.References.Spec.Tags {
		repo, ok := jsonOutput.References.Spec.Tags[i].Annotations["io.openshift.build.source-location"]
		if !ok || len(repo) == 0 {
			continue
		}
		result = append(result, repo)
	}
	return result, nil
}
