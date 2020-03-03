package events

import (
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func PrintComponents(writer io.Writer, events []*corev1.Event) error {
	components := sets.NewString()
	for _, event := range events {
		if !components.Has(event.Source.Component) {
			components.Insert(event.Source.Component)
		}
	}

	if _, err := fmt.Fprintln(writer, strings.Join(components.List(), ",")); err != nil {
		return err
	}

	return nil
}

func PrintEvents(writer io.Writer, events []*corev1.Event) error {
	for _, event := range events {
		message := event.Message
		message = strings.Replace(message, "\\\\", "\\", -1)
		message = strings.Replace(message, "\\n", "\n\t", -1)
		message = strings.Replace(message, "\\", "\"", -1)
		message = strings.Replace(message, `"""`, `"`, -1)
		message = strings.Replace(message, "\t", "\t", -1)

		countMessage := fmt.Sprintf("%d", event.Count)
		if event.Count > 1 {
			// eventDuration represents the time between first and last event observed
			if eventDuration := event.LastTimestamp.Time.Sub(event.FirstTimestamp.Time); eventDuration > 0 {
				countMessage = fmt.Sprintf("%sx %s", countMessage, event.LastTimestamp.Time.Sub(event.FirstTimestamp.Time))
			}
		}
		componentName := event.InvolvedObject.Namespace
		if len(componentName) == 0 {
			componentName = event.InvolvedObject.Name
		}
		if len(componentName) == 0 && len(event.ReportingController) > 0 || len(event.ReportingInstance) > 0 {
			componentName = fmt.Sprintf("%s-%s", event.ReportingController, event.ReportingInstance)
		}

		if _, err := fmt.Fprintf(writer, "%s (%s) %q %s %s\n", event.LastTimestamp.Format("15:04:05"), countMessage, componentName, event.Reason, message); err != nil {
			return err
		}
	}

	return nil
}

func PrintEventsWide(writer io.Writer, events []*corev1.Event) error {
	return PrintEvents(writer, events)
}
