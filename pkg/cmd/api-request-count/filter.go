package api_request_count

import (
	apiv1 "github.com/openshift/api/apiserver/v1"
)

type apiRequestFilter func([]apiv1.PerResourceAPIRequestLog) []apiv1.PerResourceAPIRequestLog

type apiRequestFilters []apiRequestFilter

func (f apiRequestFilters) apply(resources []apiv1.PerResourceAPIRequestLog) []apiv1.PerResourceAPIRequestLog {
	for _, filter := range f {
		resources = filter(resources)
	}
	return resources
}

func filterByVerbs(verbs []string) apiRequestFilter {
	return func(requests []apiv1.PerResourceAPIRequestLog) []apiv1.PerResourceAPIRequestLog {
		requestsFilter := apiRequestLogFilter{requests: requests}
		return requestsFilter.ByVerb(verbs).Filter()
	}
}

type apiRequestLogFilter struct {
	requests []apiv1.PerResourceAPIRequestLog
	nodes    []string
	users    []string
	verbs    []string
}

func (a *apiRequestLogFilter) ByNode(nodes []string) *apiRequestLogFilter {
	a.nodes = nodes
	return a
}

func (a *apiRequestLogFilter) ByUser(users []string) *apiRequestLogFilter {
	a.users = users
	return a
}

func (a *apiRequestLogFilter) ByVerb(verbs []string) *apiRequestLogFilter {
	a.verbs = verbs
	return a
}

func (a *apiRequestLogFilter) Filter() []apiv1.PerResourceAPIRequestLog {
	ret := []apiv1.PerResourceAPIRequestLog{}
	for _, request := range a.requests {
		filteredRequest, filteredRequestTotal := filterCountNodesUsersVerbs(a.nodes, a.verbs, a.verbs, request.ByNode)
		ret = append(ret, apiv1.PerResourceAPIRequestLog{
			ByNode:       filteredRequest,
			RequestCount: filteredRequestTotal,
		})
	}
	return ret
}

func filterCountNodesUsersVerbs(nodes, users, verbs []string, requestsFromNodes []apiv1.PerNodeAPIRequestLog) ([]apiv1.PerNodeAPIRequestLog, int64) {
	ret := []apiv1.PerNodeAPIRequestLog{}
	var totalNodesRequest int64
	for _, requestsFromNode := range requestsFromNodes {
		// TODO filter by nodename
		usersRequests, totalUsersRequests := filterCountUsersVerbs(users, verbs, requestsFromNode.ByUser)
		ret = append(ret, apiv1.PerNodeAPIRequestLog{
			NodeName:     requestsFromNode.NodeName,
			RequestCount: totalUsersRequests,
			ByUser:       usersRequests,
		})
		totalNodesRequest = totalNodesRequest + totalUsersRequests
	}
	return ret, totalNodesRequest
}

func filterCountUsersVerbs(users, verbs []string, requestsFromUsers []apiv1.PerUserAPIRequestCount) ([]apiv1.PerUserAPIRequestCount, int64) {
	ret := []apiv1.PerUserAPIRequestCount{}
	var totalUsersRequest int64
	for _, requestsFromUser := range requestsFromUsers {
		// TODO filter by userName, userAgent
		filteredRequestsByVerb, filteredRequestsByVerbTotal := filterCountVerbs(verbs, requestsFromUser.ByVerb)
		ret = append(ret, apiv1.PerUserAPIRequestCount{
			UserName:     requestsFromUser.UserName,
			UserAgent:    requestsFromUser.UserAgent,
			RequestCount: filteredRequestsByVerbTotal,
			ByVerb:       filteredRequestsByVerb,
		})
		totalUsersRequest = totalUsersRequest + filteredRequestsByVerbTotal
	}
	return ret, totalUsersRequest
}

func filterCountVerbs(verbs []string, allVerbRequests []apiv1.PerVerbAPIRequestCount) ([]apiv1.PerVerbAPIRequestCount, int64) {
	ret := []apiv1.PerVerbAPIRequestCount{}
	var totalCount int64
	for _, verbResource := range allVerbRequests {
		for _, verb := range verbs {
			if verbResource.Verb == verb {
				ret = append(ret, verbResource)
				totalCount = totalCount + verbResource.RequestCount
			}
		}
	}

	return ret, totalCount
}
