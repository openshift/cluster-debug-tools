package audit

import (
	"fmt"
	"testing"
)

type tuple struct {
	key    string
	length int
}

func TestGroupBySort(t *testing.T) {
	type tuple struct {
		key    string
		length int
	}
	tests := []struct {
		name   string
		paths  []string
		output []tuple
	}{
		{
			name:  "group by username, sort and check the top 10",
			paths: []string{"user.username"},
			output: []tuple{
				{key: "system:admin", length: 24740},
				{key: "system:serviceaccount:openshift-apiserver:openshift-apiserver-sa", length: 11439},
				{key: "system:serviceaccount:openshift-kube-apiserver-operator:kube-apiserver-operator", length: 11262},
				{key: "system:serviceaccount:openshift-kube-scheduler-operator:openshift-kube-scheduler-operator", length: 9413},
				{key: "system:apiserver", length: 7772},
				{key: "system:serviceaccount:openshift-kube-controller-manager-operator:kube-controller-manager-operator", length: 7495},
				{key: "system:serviceaccount:openshift-kube-apiserver:check-endpoints", length: 5348},
				{key: "system:serviceaccount:openshift-kube-apiserver:localhost-recovery-client", length: 5205},
				{key: "system:serviceaccount:openshift-must-gather-b6p4s:default", length: 3855},
				{key: "system:serviceaccount:openshift-kube-storage-version-migrator-operator:kube-storage-version-migrator-operator", length: 3131},
			},
		},
		{
			name:  "",
			paths: []string{"verb", "requestURI", "user.username"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := GetEvents("./testdata/audit.log")
			if err != nil {
				t.Fatalf("unable to read test data, err %v", err)
			}
			aggregatedEvents := GroupBy(tt.paths, events)
			sortedEvents := SortByLen(aggregatedEvents)
			if len(aggregatedEvents) < len(tt.output) {
				t.Fatalf("the number of actual events %d (distinct keys) is less than expected %d", len(aggregatedEvents), len(tt.output))
			}
			for i := 0; i < len(tt.output); i++ {
				if tt.output[i].key != sortedEvents[i] {
					t.Fatalf("incorrect length at index %d, expected %s, got %s", i, tt.output[i].key, sortedEvents[i])
				}

				expectedLen := tt.output[i].length
				actualLen := len(aggregatedEvents[sortedEvents[i]])
				if actualLen != expectedLen {
					t.Fatalf("expected to found %d entries, got %d, for %s key", expectedLen, actualLen, sortedEvents[i])
				}
			}
		})
	}
}

func TestExampleTop5ByVerbURIAndUserName(t *testing.T) {
	groupBy := []string{"verb", "requestURI", "user.username"}
	events, err := GetEvents("./testdata/audit.log")
	if err != nil {
		t.Fatalf("unable to read test data, err %v", err)
	}
	aggregatedEvents := GroupBy(groupBy, events)
	sortedEvents := SortByLen(aggregatedEvents)

	topByVerb := map[string][]tuple{}

	for i := 0; i < len(sortedEvents); i++ {
		if events := aggregatedEvents[sortedEvents[i]]; len(events) > 0 {
			event := events[0]
			switch event.Verb {
			case "get":
				topByVerb["get"] = append(topByVerb["get"], tuple{key: fmt.Sprintf("[%s][%s]", event.RequestURI, event.User.Username), length: len(events)})
			case "create":
				topByVerb["create"] = append(topByVerb["create"], tuple{key: fmt.Sprintf("[%s][%s]", event.RequestURI, event.User.Username), length: len(events)})
			case "list":
				topByVerb["list"] = append(topByVerb["list"], tuple{key: fmt.Sprintf("[%s][%s]", event.RequestURI, event.User.Username), length: len(events)})
			case "delete":
				topByVerb["delete"] = append(topByVerb["delete"], tuple{key: fmt.Sprintf("[%s][%s]", event.RequestURI, event.User.Username), length: len(events)})
			case "update":
				topByVerb["update"] = append(topByVerb["update"], tuple{key: fmt.Sprintf("[%s][%s]", event.RequestURI, event.User.Username), length: len(events)})
			}
		}

		counter := 0
		for _, topEvents := range topByVerb {
			if len(topEvents) >= 5 {
				counter++
			}
		}

		if counter >= len(topByVerb) {
			break
		}
	}

	// validate
	expectedTopByVerb := map[string][]tuple{
		"get": {
			// Top 5 "GET":
			{key: "[/api/v1/namespaces/default/services/docker-registry][system:serviceaccount:openshift-apiserver:openshift-apiserver-sa]", length: 3150},
			{key: "[/apis/operator.openshift.io/v1/openshiftcontrollermanagers/cluster][system:serviceaccount:openshift-controller-manager-operator:openshift-controller-manager-operator]", length: 1327},
			{key: "[/api/v1/namespaces/openshift-kube-scheduler/serviceaccounts/localhost-recovery-client][system:serviceaccount:openshift-kube-scheduler-operator:openshift-kube-scheduler-operator]", length: 1029},
			{key: "[/apis/apps.openshift.io/v1/namespaces/e2e-test-cli-deployment-lgjdh/deploymentconfigs/history-limit][e2e-test-cli-deployment-lgjdh-user]", length: 853},
			{key: "[/api/v1/namespaces/openshift-kube-scheduler/configmaps/kube-scheduler?timeout=10s][system:kube-scheduler]", length: 822},
		},
		"create":{
			// Top 5 "CREATE":
			{key: "[/apis/authorization.k8s.io/v1/subjectaccessreviews][system:serviceaccount:openshift-apiserver:openshift-apiserver-sa]", length: 3215},
			{key: "[/apis/authorization.k8s.io/v1/subjectaccessreviews][system:serviceaccount:openshift-oauth-apiserver:oauth-apiserver-sa]", length: 1125},
			{key: "[/api/v1/namespaces][system:admin]", length: 296},
			{key: "[/api/v1/namespaces/e2e-webhook-8183-markers/configmaps][system:admin]", length: 278},
			{key: "[/api/v1/namespaces/kube-system/pods?dryRun=All][system:admin]", length: 270},
		},
	}

	for verb, expectedTop5ByVerbURIUser := range expectedTopByVerb {
		actualTop5ByVerbURIUser := topByVerb[verb]
		for index, expectedTuple := range expectedTop5ByVerbURIUser {
			actualTuple := actualTop5ByVerbURIUser[index]
			if actualTuple.key != expectedTuple.key {
				t.Errorf("unexpected key %q under %d index, expected %q", actualTuple.key, index, expectedTuple.key)
			}

			if actualTuple.length != expectedTuple.length {
				t.Errorf("unexpected lenght %d under %d index, expected %d", actualTuple.length, index, expectedTuple.length)
			}
		}
	}
}
