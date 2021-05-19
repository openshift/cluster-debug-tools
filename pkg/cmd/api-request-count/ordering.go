package api_request_count

import (
	"bytes"
	"sort"
	"encoding/json"
)

// primaryKeyOrder sums and sorts all nested values returning only primary keys
func primaryKeyOrder(data map[string]map[string]int64) []string {
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

func secondaryKeyOrder(data map[string]int64) []string {
	type kv struct {
		key string
		val int64
	}

	keys := make([]kv, 0, len(data))
	for key, val := range data {
		keys = append(keys, kv{key: key, val: val})
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i].val > keys[j].val })

	ret := make([]string, len(keys))
	for index, kv := range keys {
		ret[index] = kv.key
	}

	return ret
}

type kv struct {
	Key string
	Val interface{}
}

type orderedMap []kv

func (oMap orderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("{")
	for index, kv := range oMap {
		if index != 0 {
			buf.WriteString(",")
		}

		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key);
		buf.WriteString(":")

		val, err := json.Marshal(kv.Val)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
}
