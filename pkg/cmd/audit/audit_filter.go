package audit

import (
	"strings"
	"time"

	"github.com/openshift/cluster-debug-tools/pkg/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type EventFilterPredicate interface {
	Matches(*auditv1.Event) bool
}

type AuditFilters []EventFilterPredicate

func (f AuditFilters) FilterEvents(events ...*auditv1.Event) []*auditv1.Event {
	ret := make([]*auditv1.Event, len(events))
	copy(ret, events)

	for _, filter := range f {
		ret = filterEvents(filter, ret...)
	}

	return ret
}

func filterEvents(predicate EventFilterPredicate, events ...*auditv1.Event) []*auditv1.Event {
	ret := []*auditv1.Event{}
	for i := range events {
		event := events[i]
		if predicate.Matches(event) {
			ret = append(ret, event)
		}
	}
	return ret
}

type FilterByFailures struct {
}

func (f *FilterByFailures) Matches(event *auditv1.Event) bool {
	if event.ResponseStatus == nil {
		return false
	}

	return event.ResponseStatus.Code > 299
}

type FilterByHTTPStatus struct {
	HTTPStatusCodes sets.Int32
}

func (f *FilterByHTTPStatus) Matches(event *auditv1.Event) bool {
	if event.ResponseStatus == nil {
		return false
	}

	if f.HTTPStatusCodes.Has(event.ResponseStatus.Code) {
		return true
	}
	return false
}

type FilterByNamespaces struct {
	Namespaces sets.String
}

func (f *FilterByNamespaces) Matches(event *auditv1.Event) bool {
	ns, _, _, _ := URIToParts(event.RequestURI)

	return util.AcceptString(f.Namespaces, ns)

}

type FilterBySubresources struct {
	Subresources sets.String
}

func (f *FilterBySubresources) Matches(event *auditv1.Event) bool {
	_, _, _, subresource := URIToParts(event.RequestURI)

	if f.Subresources.Has("-*") && len(f.Subresources) == 1 && len(subresource) == 0 {
		return true
	}

	return util.AcceptString(f.Subresources, subresource)
}

type FilterByNames struct {
	Names sets.String
}

func (f *FilterByNames) Matches(event *auditv1.Event) bool {
	_, _, name, _ := URIToParts(event.RequestURI)

	if util.AcceptString(f.Names, name) {
		return true
	}

	// if we didn't match, check the objectref
	if event.ObjectRef == nil {
		return false
	}

	return util.AcceptString(f.Names, event.ObjectRef.Name)
}

type FilterByUIDs struct {
	UIDs sets.String
}

func (f *FilterByUIDs) Matches(event *auditv1.Event) bool {
	return util.AcceptString(f.UIDs, string(event.AuditID))

}

type FilterByUser struct {
	Users sets.String
}

func (f *FilterByUser) Matches(event *auditv1.Event) bool {
	return util.AcceptString(f.Users, event.User.Username)
}

type FilterByVerbs struct {
	Verbs sets.String
}

func (f *FilterByVerbs) Matches(event *auditv1.Event) bool {
	return util.AcceptString(f.Verbs, event.Verb)
}

type FilterByResources struct {
	Resources map[schema.GroupResource]bool
}

func (f *FilterByResources) Matches(event *auditv1.Event) bool {
	_, gvr, _, _ := URIToParts(event.RequestURI)
	antiMatch := schema.GroupResource{Resource: "-" + gvr.Resource, Group: gvr.Group}

	// check for an anti-match
	if f.Resources[antiMatch] {
		return false
	}
	if f.Resources[gvr.GroupResource()] {
		return true
	}

	// if we aren't an exact match, match on resource only if group is '*'
	// check for an anti-match
	for currResource := range f.Resources {
		if currResource.Group == "*" && currResource.Resource == antiMatch.Resource {
			return false
		}
		if currResource.Resource == "-*" && currResource.Group == gvr.Group {
			return false
		}
	}

	for currResource := range f.Resources {
		if currResource.Group == "*" && currResource.Resource == "*" {
			return true
		}
		if currResource.Group == "*" && currResource.Resource == gvr.Resource {
			return true
		}
		if currResource.Resource == "*" && currResource.Group == gvr.Group {
			return true
		}
	}

	return false
}

func URIToParts(uri string) (string, schema.GroupVersionResource, string, string) {
	ns := ""
	gvr := schema.GroupVersionResource{}
	name := ""

	if len(uri) >= 1 {
		if uri[0] == '/' {
			uri = uri[1:]
		}
	}

	// some request URL has query parameters like: /apis/image.openshift.io/v1/images?limit=500&resourceVersion=0
	// we are not interested in the query parameters.
	uri = strings.Split(uri, "?")[0]
	parts := strings.Split(uri, "/")
	if len(parts) == 0 {
		return ns, gvr, name, ""
	}
	// /api/v1/namespaces/<name>
	if parts[0] == "api" {
		if len(parts) >= 2 {
			gvr.Version = parts[1]
		}
		if len(parts) < 3 {
			return ns, gvr, name, ""
		}

		switch {
		case parts[2] != "namespaces": // cluster scoped request that is not a namespace
			gvr.Resource = parts[2]
			if len(parts) >= 4 {
				name = parts[3]
				return ns, gvr, name, ""
			}
		case len(parts) == 3 && parts[2] == "namespaces": // a namespace request /api/v1/namespaces
			gvr.Resource = parts[2]
			return "", gvr, "", ""

		case len(parts) == 4 && parts[2] == "namespaces": // a namespace request /api/v1/namespaces/<name>
			gvr.Resource = parts[2]
			name = parts[3]
			ns = parts[3]
			return ns, gvr, name, ""

		case len(parts) == 5 && parts[2] == "namespaces" && parts[4] == "finalize", // a namespace request /api/v1/namespaces/<name>/finalize
			len(parts) == 5 && parts[2] == "namespaces" && parts[4] == "status": // a namespace request /api/v1/namespaces/<name>/status
			gvr.Resource = parts[2]
			name = parts[3]
			ns = parts[3]
			return ns, gvr, name, parts[4]

		default:
			// this is not a cluster scoped request and not a namespace request we recognize
		}

		if len(parts) < 4 {
			return ns, gvr, name, ""
		}

		ns = parts[3]
		if len(parts) >= 5 {
			gvr.Resource = parts[4]
		}
		if len(parts) >= 6 {
			name = parts[5]
		}
		if len(parts) >= 7 {
			return ns, gvr, name, strings.Join(parts[6:], "/")
		}
		return ns, gvr, name, ""
	}

	if parts[0] != "apis" {
		return ns, gvr, name, ""
	}

	// /apis/group/v1/namespaces/<name>
	if len(parts) >= 2 {
		gvr.Group = parts[1]
	}
	if len(parts) >= 3 {
		gvr.Version = parts[2]
	}
	if len(parts) < 4 {
		return ns, gvr, name, ""
	}

	if parts[3] != "namespaces" {
		gvr.Resource = parts[3]
		if len(parts) >= 5 {
			name = parts[4]
			return ns, gvr, name, ""
		}
	}
	if len(parts) < 5 {
		return ns, gvr, name, ""
	}

	ns = parts[4]
	if len(parts) >= 6 {
		gvr.Resource = parts[5]
	}
	if len(parts) >= 7 {
		name = parts[6]
	}
	if len(parts) >= 8 {
		return ns, gvr, name, strings.Join(parts[7:], "/")
	}
	return ns, gvr, name, ""
}

type FilterByStage struct {
	Stages sets.String
}

func (f *FilterByStage) Matches(event *auditv1.Event) bool {
	// TODO: an event not having a stage, what do we do?
	return f.Stages.Has(string(event.Stage))
}

type FilterByAfter struct {
	After time.Time
}

func (f *FilterByAfter) Matches(event *auditv1.Event) bool {
	return event.RequestReceivedTimestamp.After(f.After)
}

type FilterByBefore struct {
	Before    time.Time
	microtime metav1.MicroTime
}

func (f *FilterByBefore) Matches(event *auditv1.Event) bool {
	if f.microtime.IsZero() {
		f.microtime = metav1.NewMicroTime(f.Before)
	}
	return event.RequestReceivedTimestamp.Before(&f.microtime)
}

type FilterByDuration struct {
	Duration time.Duration
}

func (f *FilterByDuration) Matches(event *auditv1.Event) bool {
	return event.StageTimestamp.Sub(event.RequestReceivedTimestamp.Time) <= f.Duration
}
