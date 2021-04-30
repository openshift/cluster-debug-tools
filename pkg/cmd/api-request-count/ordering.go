package api_request_count

import "sort"

// sortByPrimaryKey sums and sorts all nested values returning only primary keys
func sortByPrimaryKey(data map[string]map[string]int64) []string {
	type kv struct {
		key string
		sum int64
	}

	primaryKeys := make([]kv, 0, len(data))
	for primaryKey, nestedMap := range data {
		var nestedValueSum int64
		for _, nestedValue := range nestedMap {
			nestedValueSum = nestedValueSum + nestedValue
		}
		primaryKeys = append(primaryKeys, kv{key: primaryKey, sum: nestedValueSum})
	}

	sort.Slice(primaryKeys, func(i, j int) bool { return primaryKeys[i].sum > primaryKeys[j].sum })

	ret := make([]string, len(primaryKeys))
	for index, kv := range primaryKeys {
		ret[index] = kv.key
	}

	return ret
}
