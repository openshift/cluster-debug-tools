package audit

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: should not be part of this file/pkg
type latencyTrackerTuple struct {
	latency   time.Duration
	timestamp metav1.MicroTime
}

type slidingWindowIterator struct {
	data           []latencyTrackerTuple
	windowDuration time.Duration
	stepDuration   time.Duration
	currentIndex   int
	stepIndex      int
	windowStart    time.Time
	windowEnd      time.Time
}

func NewSlidingWindowIterator(data []latencyTrackerTuple, windowDuration, stepDuration time.Duration) *slidingWindowIterator {
	if len(data) == 0 {
		return &slidingWindowIterator{}
	}

	return &slidingWindowIterator{
		data:           data,
		windowDuration: windowDuration,
		stepDuration:   stepDuration,
		currentIndex:   0,
		stepIndex:      0,
		windowStart:    data[0].timestamp.Time,
		windowEnd:      data[0].timestamp.Time.Add(windowDuration),
	}
}

func (it *slidingWindowIterator) Next() []latencyTrackerTuple {
	if it.currentIndex >= len(it.data) {
		return nil
	}

	var windowData []latencyTrackerTuple
	for i := it.currentIndex; i < len(it.data); i++ {
		entry := it.data[i]
		if entry.timestamp.Time.After(it.windowEnd) {
			break
		}

		if entry.timestamp.Time.After(it.windowStart) || entry.timestamp.Time.Equal(it.windowStart) {
			windowData = append(windowData, entry)
		}

		if !entry.timestamp.Time.After(it.windowStart.Add(it.stepDuration)) {
			it.stepIndex = i
		}
	}

	it.windowStart = it.windowStart.Add(it.stepDuration)
	it.windowEnd = it.windowStart.Add(it.windowDuration)
	it.currentIndex = it.stepIndex

	return windowData
}
