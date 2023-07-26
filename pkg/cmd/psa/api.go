package psa

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	psapi "k8s.io/pod-security-admission/api"
)

const labelSyncControlLabel = "security.openshift.io/scc.podSecurityLabelSync"

// PodSecurityViolation is a violation of the PodSecurity level set.
type PodSecurityViolation struct {
	metav1.TypeMeta `json:",inline"`

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

	// Labels contain the labels of interest, present in the namespace.
	Labels map[string]psapi.Level `json:"pslabels,omitempty"`
	// IsSyncControlLabel signals that the label syncer is turned on for this namespace.
	IsSyncControlLabel bool `json:"isSyncControlLabel,omitempty"`
}

// Ensure PodSecurityViolation implements the runtime.Object interface.
var _ runtime.Object = &PodSecurityViolation{}

// DeepCopyObject complements the runtime.Object interface.
func (v *PodSecurityViolation) DeepCopyObject() runtime.Object {
	if v == nil {
		return nil
	}

	c := &PodSecurityViolation{}

	c.TypeMeta = v.TypeMeta
	c.Namespace = v.Namespace
	c.Level = v.Level
	c.PodName = v.PodName
	c.IsSyncControlLabel = v.IsSyncControlLabel

	if v.Pod != nil {
		c.Pod = v.Pod.DeepCopy()
	}

	if v.Labels != nil {
		c.Labels = make(map[string]psapi.Level, len(v.Labels))
		for k, v := range v.Labels {
			c.Labels[k] = v
		}
	}

	c.Violations = make([]string, len(v.Violations))
	copy(c.Violations, v.Violations)
	c.PodControllers = make([]any, len(v.PodControllers))
	copy(c.PodControllers, v.PodControllers)

	return c

}

// PodSecurityViolationList is a list of PodSecurityViolation objects.
type PodSecurityViolationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is a list of PodSecurityViolation objects.
	Items []PodSecurityViolation `json:"items"`
}

// Ensure PodSecurityViolationList implements the runtime.Object interface.
var _ runtime.Object = &PodSecurityViolationList{}

// DeepCopyObject complements the runtime.Object interface.
func (l *PodSecurityViolationList) DeepCopyObject() runtime.Object {
	if l == nil {
		return nil
	}

	out := PodSecurityViolationList{}
	out.TypeMeta = l.TypeMeta
	out.ListMeta = *l.ListMeta.DeepCopy()

	if l.Items == nil {
		return &out
	}

	out.Items = make([]PodSecurityViolation, len(l.Items))
	for i := range l.Items {
		p := l.Items[i].DeepCopyObject().(*PodSecurityViolation)
		out.Items[i] = *p
	}

	return &out
}
