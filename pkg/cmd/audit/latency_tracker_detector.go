package audit

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func DetectIncreasedLatencyFromAnalysedEventsWithLatencyTrackerSpans(analysedEventsWithLatencyTrackerSpans []*AnalysedEventsWithLatencyTrackerSpan, latencyTracker string) {
	var spansWithIncreasedLatency []*AnalysedEventsWithLatencyTrackerSpan
	var currentSpan *AnalysedEventsWithLatencyTrackerSpan
	increasedLatencyThreshold := 2 * time.Second

	for _, analysedEventsWithLatencyTrackerSpan := range analysedEventsWithLatencyTrackerSpans {
		if analysedEventsWithLatencyTrackerSpan.Median >= increasedLatencyThreshold {
			if currentSpan == nil {
				currentSpan = analysedEventsWithLatencyTrackerSpan
				continue
			}
			if analysedEventsWithLatencyTrackerSpan.FirstEventTimeStamp.Before(&currentSpan.LastEventTimeStamp) || analysedEventsWithLatencyTrackerSpan.FirstEventTimeStamp.Equal(&currentSpan.LastEventTimeStamp) {
				spansWithIncreasedLatency = append(spansWithIncreasedLatency, analysedEventsWithLatencyTrackerSpan)
				currentSpan = analysedEventsWithLatencyTrackerSpan
				continue
			}
			printIncreasedLatencyFor(latencyTracker, spansWithIncreasedLatency)
			currentSpan = analysedEventsWithLatencyTrackerSpan
			spansWithIncreasedLatency = nil
		}
	}
	printIncreasedLatencyFor(latencyTracker, spansWithIncreasedLatency)
	printIncreasedLatencyFor(latencyTracker, []*AnalysedEventsWithLatencyTrackerSpan{currentSpan})
}

func printIncreasedLatencyFor(latencyTracker string, spansWithIncreasedLatency []*AnalysedEventsWithLatencyTrackerSpan) {
	if len(spansWithIncreasedLatency) == 0 {
		return
	}

	var totalEvents int
	var firstEventTime metav1.MicroTime
	var lastEventTime metav1.MicroTime

	firstEventTime = spansWithIncreasedLatency[0].FirstEventTimeStamp
	for _, spanWithIncreasedLatency := range spansWithIncreasedLatency {
		totalEvents = totalEvents + len(spanWithIncreasedLatency.Events)
		lastEventTime = spanWithIncreasedLatency.LastEventTimeStamp
	}

	fmt.Println("")
	fmt.Println(fmt.Sprintf("increased latency for tracker: %s detected, from: %v, to: %v, totalEvents: %v\n", latencyTracker, firstEventTime, lastEventTime, totalEvents))
	for _, spanWithIncreasedLatency := range spansWithIncreasedLatency {
		fmt.Println(fmt.Sprintf("tracker=%v, min=%v max=%v median=%v 90th=%v events=%v [%v - %v]\n",
			latencyTracker,
			spanWithIncreasedLatency.Min,
			spanWithIncreasedLatency.Max,
			spanWithIncreasedLatency.Median,
			spanWithIncreasedLatency.P90,
			len(spanWithIncreasedLatency.Events),
			spanWithIncreasedLatency.FirstEventTimeStamp,
			spanWithIncreasedLatency.LastEventTimeStamp))
	}
}
