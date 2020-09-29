package audit

import (
	"fmt"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type auditKeyGroup struct {
	key   string
	group []*auditv1.Event
}

type iterator func() (*auditKeyGroup, bool, error)
type Predicate func(ae []*auditv1.Event) []*auditv1.Event

type Query struct {
	getIterator func() iterator
}

func Read(fileName string) *Query {
	return &Query{
		getIterator: func() iterator {
			allEvents, err := GetEvents(fileName)
			if err != nil {
				// TODO: to err function
				return func() (*auditKeyGroup, bool, error) {
					return nil, false, err
				}
			}
			noMoreData := false

			return func() (*auditKeyGroup, bool, error) {
				if noMoreData {
					return nil, false, nil
				}
				aes := &auditKeyGroup{key: "", group: allEvents}
				noMoreData = true
				return aes, true, nil
			}
		},
	}
}

func (q *Query) Filter(predicate Predicate) *Query {
	return &Query{
		getIterator: func() iterator {
			iter := q.getIterator()

			return func() (*auditKeyGroup, bool, error) {
				for {
					aes, hasMore, err := iter()
					if !hasMore || err != nil {
						return aes, hasMore, err
					}

					filteredEvents := predicate(aes.group)
					if len(filteredEvents) == 0 {
						continue
					}
					return &auditKeyGroup{key: aes.key, group: filteredEvents}, true, nil
				}
			}
		},
	}
}

func (q *Query) SortBy(query string) *Query {
	return &Query{
		getIterator: func() iterator {
			if query != "length" {
				// TODO: to err function
				return func() (*auditKeyGroup, bool, error) {
					return nil, false, fmt.Errorf("currently only \"length\" query is supported, got %q", query)
				}
			}
			iter := q.getIterator()
			aggregatedEvents := map[string][]*auditv1.Event{}
			for {
				aes, hasMore, err := iter()
				if err != nil {
					// TODO: to err function
					return func() (*auditKeyGroup, bool, error) {
						return nil, false, err
					}
				}
				if !hasMore {
					break
				}
				aggregatedEvents[aes.key] = aes.group
			}
			sortedEventKeys := SortByLen(aggregatedEvents)
			index := 0

			return func() (*auditKeyGroup, bool, error) {
				if index == len(sortedEventKeys) {
					return nil, false, nil
				}
				key := sortedEventKeys[index]
				aes := &auditKeyGroup{key: key, group: aggregatedEvents[key]}
				index++
				return aes, true, nil
			}
		},
	}
}

func (q *Query) GroupBy(query []string) *Query {
	return &Query{
		getIterator: func() iterator {
			iter := q.getIterator()
			allEvents := []*auditv1.Event{}
			for {
				aes, hasMore, err := iter()
				if err != nil {
					// TODO: to err function
					return func() (*auditKeyGroup, bool, error) {
						return nil, false, err
					}
				}
				if !hasMore {
					break
				}
				allEvents = append(allEvents, aes.group...)
			}

			aggregatedEvents := GroupBy(query, allEvents)
			aggregatedEventsKeys := make([]string, len(aggregatedEvents))
			for key, _ := range aggregatedEvents {
				aggregatedEventsKeys = append(aggregatedEventsKeys, key)
			}
			index := 0

			return func() (*auditKeyGroup, bool, error) {
				if index == len(aggregatedEventsKeys) {
					return nil, false, nil
				}
				aes := &auditKeyGroup{key: aggregatedEventsKeys[index], group: aggregatedEvents[aggregatedEventsKeys[index]]}
				index++
				return aes, true, nil
			}
		},
	}
}

type Result struct {
	Key   string
	Group []*auditv1.Event
}

func (q *Query) Run() ([]*Result, error) {
	iter := q.getIterator()
	ret := []*Result{}
	for {
		aes, hasMore, err := iter()
		if err != nil {
			return nil, err
		}
		if !hasMore {
			break
		}
		ret = append(ret, &Result{Key: aes.key, Group: aes.group})
	}

	return ret, nil
}
