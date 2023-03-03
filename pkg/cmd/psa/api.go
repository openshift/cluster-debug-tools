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
	// PodViolations lists the pods that violate the PodSecurity level.
	PodViolations []*PodViolation `json:"podViolations"`
}

type PodViolation struct {
	// PodName is the name of the pod that violates the PodSecurity level.
	PodName string `json:"podName"`
	// Violations lists the violations that the pod has.
	Violations []string `json:"violations"`
	// Pod is the pod that violates the PodSecurity level.
	Pod *corev1.Pod `json:"pod,omitempty"`
	// PodController is the controller that manages the pod.
	PodController any `json:"deployment,omitempty"`
}
