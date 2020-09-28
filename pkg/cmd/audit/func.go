package audit

import (
	"sort"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func SortByLen(events map[string][]*auditv1.Event) []string {
	keys := make([]string, 0, len(events))
	for k, _ := range events {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(events[keys[i]]) >= len(events[keys[j]])
	})
	return keys
}
