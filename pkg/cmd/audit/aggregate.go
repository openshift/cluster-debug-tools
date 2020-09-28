package audit

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/audit"
)

func GroupBy(paths []string,  events []*auditv1.Event) map[string][]*auditv1.Event {
	result := map[string][]*auditv1.Event{}
	for _, event := range events {
		unstructuredEvent := map[string]interface{}{}
		{
			asUnstructured := &unstructured.Unstructured{}
			if err := audit.Scheme.Convert(event, asUnstructured, nil); err != nil {
				panic(err)
			}
			unstructuredEvent = asUnstructured.Object
		}

		keyParts := []string{}
		for _, path := range paths {
			value, found, err := unstructured.NestedString(unstructuredEvent, strings.Split(path, ".")...)
			if err != nil {
				panic(err)
			}
			if !found {
				continue
			}
			if len(value) == 0 {
				continue
			}
			keyParts = append(keyParts, value)
		}
		key := strings.Join(keyParts, "/")
		eventsPerKey := result[key]
		if len(eventsPerKey) == 0 {
			eventsPerKey = []*auditv1.Event{}
		}
		eventsPerKey = append(eventsPerKey, event)
		result[key] = eventsPerKey
	}

	return result
}
