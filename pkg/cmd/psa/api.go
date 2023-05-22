package psa

import (
	corev1 "k8s.io/api/core/v1"
)

// PodSecurityViolation is a violation of the PodSecurity level set.
type PodSecurityViolation struct {
	// Namespace where the violation happened.
	Namespace string `json:"namespace"`
	// Level is the pod security level that was violated.
	Level string `json:"level"`
	// PodName is the name of the pod with the shortest name that violates the
	// PodSecurity level.
	PodName string `json:"podName"`
	// Violations lists the violations that all the pods in the namespace made.
	Violations []string `json:"violations"`
	// Pod is the pod with the shortest name that violates the PodSecurity level.
	Pod *corev1.Pod `json:"pod,omitempty"`
	// PodController is the controller that manages the pod referenced.
	PodControllers []any `json:"podcontroller,omitempty"`
}
