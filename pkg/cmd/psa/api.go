package psa

import (
	appsv1 "k8s.io/api/apps/v1"
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
	// Deployment is the deployment that the pod belongs to.
	Deployment *appsv1.Deployment `json:"deployment,omitempty"`
	// Pod is the pod that violates the PodSecurity level.
	Pod *corev1.Pod `json:"pod,omitempty"`
}
