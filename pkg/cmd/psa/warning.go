package psa

import (
	"k8s.io/client-go/rest"
)

// Warnings Mapping
type warningsHandler struct {
	defaultHandler rest.WarningHandler
	warnings       []string
}

// HandleWarningHeader implements the WarningHandler interface. It stores the
// warning headers.
func (w *warningsHandler) HandleWarningHeader(code int, agent string, text string) {
	if text == "" {
		return
	}

	w.warnings = append(w.warnings, text)

	if w.defaultHandler == nil {
		return
	}

	w.defaultHandler.HandleWarningHeader(code, agent, text)
}

// PopAll returns all warnings and clears the slice.
func (w *warningsHandler) PopAll() []string {
	warnings := w.warnings
	w.warnings = []string{}

	return warnings
}
