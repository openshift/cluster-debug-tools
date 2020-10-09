package release_log

import "testing"

func TestGetReleaseImageRepositories(t *testing.T) {
	repos, err := getReleaseImageRepositories("quay.io/openshift-release-dev/ocp-release:4.6.0-rc.1-x86_64")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", repos)
}
