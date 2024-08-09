package audit

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

// TODO: should not be part of this file/pkg
type EventWithLatencyTracker struct {
	Event   *auditv1.Event
	Latency time.Duration
}

// ToSortedEventWithLatencyTracker aggregates audit events by "apiserver.latency.k8s.io" annotation and stores them in a sorted array (by RequestReceivedTimestamp).
func ToSortedEventWithLatencyTracker(events []*auditv1.Event) map[string][]*EventWithLatencyTracker {
	eventsWithLatencyTracker := map[string][]*EventWithLatencyTracker{}
	for _, event := range events {
		for latencyTracker, latencyValue := range event.Annotations {
			if !strings.HasPrefix(latencyTracker, "apiserver.latency.k8s.io/") {
				continue
			}

			latencyDuration, err := time.ParseDuration(latencyValue)
			if err != nil {
				fmt.Println(fmt.Sprintf("Error parsing %q=%v duration, err=%v", latencyTracker, latencyValue, err))
				continue
			}
			eventsWithLatencyTracker[latencyTracker] = append(eventsWithLatencyTracker[latencyTracker], &EventWithLatencyTracker{Event: event, Latency: latencyDuration})
		}
	}

	for latencyTracker, eventsWithLatencies := range eventsWithLatencyTracker {
		sort.Slice(eventsWithLatencies, func(i, j int) bool {
			return eventsWithLatencies[i].Event.RequestReceivedTimestamp.Before(&eventsWithLatencies[j].Event.RequestReceivedTimestamp)
		})
		eventsWithLatencyTracker[latencyTracker] = eventsWithLatencies
	}

	return eventsWithLatencyTracker
}

func EventWithLatencyTrackerToDuration(eventsWithLatencyTracker []*EventWithLatencyTracker) []time.Duration {
	latencies := make([]time.Duration, len(eventsWithLatencyTracker))
	for i, eventWithLatencyTracker := range eventsWithLatencyTracker {
		latencies[i] = eventWithLatencyTracker.Latency
	}
	return latencies
}

func IsWholeNumber(num float64) bool {
	return num == math.Floor(num)
}

func Mean(latency1, latency2 time.Duration) time.Duration {
	latency1Ns := latency1.Nanoseconds()
	latency2Ns := latency2.Nanoseconds()
	meanLatencyNs := (latency1Ns + latency2Ns) / 2
	return time.Duration(meanLatencyNs)
}

func Median(latencies []time.Duration) time.Duration {
	var median time.Duration
	if len(latencies)%2 == 0 {
		latencies = latencies[len(latencies)/2-1 : len(latencies)/2+1]
		median = Mean(latencies[0], latencies[1])
	} else {
		median = latencies[len(latencies)/2]
	}
	return median
}

func Percentile(percentile float64, latencies []time.Duration) time.Duration {
	indexForPercentile := (percentile / 100.0) * float64(len(latencies))
	if IsWholeNumber(indexForPercentile) {
		return latencies[int(indexForPercentile)]
	}
	if indexForPercentile > 1 {
		return Mean(latencies[int(indexForPercentile)-1], latencies[int(indexForPercentile)])
	}
	return 0
}
