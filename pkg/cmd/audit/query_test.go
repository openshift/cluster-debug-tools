package audit_test

import (
	"github.com/openshift/cluster-debug-tools/pkg/cmd/audit"
	"testing"
)

func TestDriveQuery(t *testing.T) {
	res, err := audit.Read("./testdata/audit.log").GroupBy([]string{"user.username"}).SortBy("length").Run()
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}

	// TODO: validate res
	_ = res
}
