package audit

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func TestGroupBy(t *testing.T) {
	var tests = []struct {
		name  string
		paths []string
		input []*auditv1.Event
		output map[string]int
	}{
		{
			name: "group by RequestURI and ObjectRef.Name",
			paths: []string{
				"requestURI",
				"objectRef.name",
			},
			input: []*auditv1.Event{
				{
					RequestURI:               "/apis/user.openshift.io/v1/users",
					UserAgent:                "agent1",
					ObjectRef:                &auditv1.ObjectReference{Name: "lukasz"},
					RequestReceivedTimestamp: metav1.NewMicroTime(time.Now()),
					StageTimestamp:           metav1.NewMicroTime(time.Now()),
				},

				{
					RequestURI:               "/apis/user.openshift.io/v1/users",
					UserAgent:                "agent1",
					ObjectRef:                &auditv1.ObjectReference{Name: "lukasz"},
					RequestReceivedTimestamp: metav1.NewMicroTime(time.Now()),
					StageTimestamp:           metav1.NewMicroTime(time.Now()),
				},

				{
					RequestURI:               "/apis/user.openshift.io/v1/users",
					UserAgent:                "agent1",
					ObjectRef:                &auditv1.ObjectReference{Name: "karolina"},
					RequestReceivedTimestamp: metav1.NewMicroTime(time.Now()),
					StageTimestamp:           metav1.NewMicroTime(time.Now()),
				},
			},
			output: map[string]int{
				"/apis/user.openshift.io/v1/users/lukasz":2,
				"/apis/user.openshift.io/v1/users/karolina":1,
			},
		},

		{
			name: "empty path for group by",
			input: []*auditv1.Event{
				{
					RequestURI:               "/apis/user.openshift.io/v1/users",
					UserAgent:                "agent1",
					ObjectRef:                &auditv1.ObjectReference{Name: "lukasz"},
					RequestReceivedTimestamp: metav1.NewMicroTime(time.Now()),
					StageTimestamp:           metav1.NewMicroTime(time.Now()),
				},

				{
					RequestURI:               "/apis/user.openshift.io/v1/users",
					UserAgent:                "agent1",
					ObjectRef:                &auditv1.ObjectReference{Name: "lukasz"},
					RequestReceivedTimestamp: metav1.NewMicroTime(time.Now()),
					StageTimestamp:           metav1.NewMicroTime(time.Now()),
				},

				{
					RequestURI:               "/apis/user.openshift.io/v1/users",
					UserAgent:                "agent1",
					ObjectRef:                &auditv1.ObjectReference{Name: "karolina"},
					RequestReceivedTimestamp: metav1.NewMicroTime(time.Now()),
					StageTimestamp:           metav1.NewMicroTime(time.Now()),
				},
			},
			output: map[string]int{
				"": 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupBy(tt.paths, tt.input)
			if len(result) != len(tt.output) {
				t.Fatalf("unexpected number of keys, exepcted %d, got %d", len(tt.output), len(result))
			}
			for expectedKey, expectedLen := range tt.output {
				actualEvents := result[expectedKey]
				if len(actualEvents) != expectedLen {
					t.Fatalf("unexpected number of events found under %s, actual %d, expected %d", expectedKey, len(actualEvents), expectedLen)
				}
			}
		})
	}
}
