package api_request_count

import (
	apiv1 "github.com/openshift/api/apiserver/v1"
)

type aggregateFn func(resourceName string, resourceRequests []apiv1.PerResourceAPIRequestLog) map[string]map[string]int64

// byUsers count resources used by users
//
// "bob": {
//   "configmaps":1,
//   "secrets":2,
//  }
func groupByUser(resourceName string, resourceRequests []apiv1.PerResourceAPIRequestLog) map[string]map[string]int64 {
	ret := map[string]map[string]int64{}

	userUsages := groupByUserName(resourceRequests)
	for user, counter := range userUsages {
		ret[user] = map[string]int64{resourceName: counter}
	}

	return ret
}

// groupByResource count users of a resource
//
// "secrets": {
//   "bob":1,
//   "alice":2,
//  }
func groupByResource(resourceName string, resourceRequests []apiv1.PerResourceAPIRequestLog) map[string]map[string]int64 {
	ret := map[string]map[string]int64{}

	userUsages := groupByUserName(resourceRequests)
	ret[resourceName] = userUsages

	return ret
}

func groupByUserName(resourceRequests []apiv1.PerResourceAPIRequestLog) map[string]int64 {
	ret := map[string]int64{}

	for _, resourceRequest := range resourceRequests {
		for _, resourceRequestByNode := range resourceRequest.ByNode {
			for _, resourceRequestByUser := range resourceRequestByNode.ByUser {
				currentRequestCount := ret[resourceRequestByUser.UserAgent]
				ret[resourceRequestByUser.UserName] = currentRequestCount + resourceRequestByUser.RequestCount
			}
		}
	}

	return ret
}

func mergeMaps(prev map[string]map[string]int64, current map[string]map[string]int64) {
	for currentKey, currentNestedMap := range current {
		prevNestedMap := prev[currentKey]
		if prevNestedMap == nil {
			prevNestedMap = map[string]int64{}
		}
		for currentNestedKey, currentNestedValue := range currentNestedMap {
			prevNestedValue := prevNestedMap[currentNestedKey]
			prevNestedMap[currentNestedKey] = prevNestedValue + currentNestedValue
		}
		prev[currentKey] = prevNestedMap
	}
}

// TODO: add support for specific range
func getRequestHistoryForTheLast(startIndex int, endIndex int, onlyCurrentHour bool, requestStatus apiv1.APIRequestCountStatus) []apiv1.PerResourceAPIRequestLog {
	if onlyCurrentHour {
		return []apiv1.PerResourceAPIRequestLog{requestStatus.CurrentHour}
	}
	ret := []apiv1.PerResourceAPIRequestLog{}
	return ret
}
