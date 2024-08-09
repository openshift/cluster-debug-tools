package audit

import (
	"time"
)

type slidingWindowIteratorForLatencyTracker struct {
	events         []*EventWithLatencyTracker
	windowDuration time.Duration
	stepDuration   time.Duration
	currentIndex   int
	stepIndex      int
	windowStart    time.Time
	windowEnd      time.Time
}

func NewSlidingWindowIteratorForLatencyTracker(events []*EventWithLatencyTracker, windowDuration, stepDuration time.Duration) *slidingWindowIteratorForLatencyTracker {
	if len(events) == 0 {
		return &slidingWindowIteratorForLatencyTracker{}
	}

	return &slidingWindowIteratorForLatencyTracker{
		events:         events,
		windowDuration: windowDuration,
		stepDuration:   stepDuration,
		currentIndex:   0,
		stepIndex:      0,
		windowStart:    events[0].Event.RequestReceivedTimestamp.Time,
		windowEnd:      events[0].Event.RequestReceivedTimestamp.Time.Add(windowDuration),
	}
}

func (it *slidingWindowIteratorForLatencyTracker) Next() ([]*EventWithLatencyTracker, bool) {
	if it.currentIndex >= len(it.events)-1 {
		return nil, false
	}

	var windowData []*EventWithLatencyTracker
	for i := it.currentIndex; i < len(it.events); i++ {
		entry := it.events[i]
		if entry.Event.RequestReceivedTimestamp.Time.After(it.windowEnd) {
			break
		}

		if entry.Event.RequestReceivedTimestamp.Time.After(it.windowStart) || entry.Event.RequestReceivedTimestamp.Time.Equal(it.windowStart) {
			windowData = append(windowData, entry)
		}

		if !entry.Event.RequestReceivedTimestamp.Time.After(it.windowStart.Add(it.stepDuration)) {
			it.stepIndex = i
		}
	}

	it.windowStart = it.windowStart.Add(it.stepDuration)
	it.windowEnd = it.windowStart.Add(it.windowDuration)
	it.currentIndex = it.stepIndex

	return windowData, true
}
