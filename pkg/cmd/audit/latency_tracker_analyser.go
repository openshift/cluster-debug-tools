package audit

import (
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AnalysedEventsWithLatencyTrackerSpan struct {
	Events []*EventWithLatencyTracker
	Min    time.Duration
	Max    time.Duration
	Median time.Duration
	P90    time.Duration

	FirstEventTimeStamp metav1.MicroTime
	LastEventTimeStamp  metav1.MicroTime
}

type latencyTrackerAnalyser struct {
	iterator       *slidingWindowIteratorForLatencyTracker
	latencyTracker string
}

func NewLatencyTrackerAnalyser(iterator *slidingWindowIteratorForLatencyTracker) *latencyTrackerAnalyser {
	return &latencyTrackerAnalyser{iterator: iterator}
}

func (l *latencyTrackerAnalyser) RunAnalysis() []*AnalysedEventsWithLatencyTrackerSpan {
	analysedEvents := []*AnalysedEventsWithLatencyTrackerSpan{}

	for {
		eventsWithLatenciesSpan, hasMoreData := l.iterator.Next()
		if !hasMoreData {
			break
		}
		if len(eventsWithLatenciesSpan) <= 1 {
			continue
		}

		latencies := EventWithLatencyTrackerToDuration(eventsWithLatenciesSpan)

		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		analysedEvents = append(analysedEvents, &AnalysedEventsWithLatencyTrackerSpan{
			Events:              eventsWithLatenciesSpan,
			Min:                 latencies[0],
			Max:                 latencies[len(latencies)-1],
			Median:              Median(latencies),
			P90:                 Percentile(90, latencies),
			FirstEventTimeStamp: eventsWithLatenciesSpan[0].Event.RequestReceivedTimestamp,
			LastEventTimeStamp:  eventsWithLatenciesSpan[len(eventsWithLatenciesSpan)-1].Event.RequestReceivedTimestamp,
		})
	}

	return analysedEvents
}
