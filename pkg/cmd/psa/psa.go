package psa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	psapi "k8s.io/pod-security-admission/api"

	"github.com/spf13/cobra"
)

// PSAOptions contains all the options and configsi for running the PSA command.
type PSAOptions struct {
	kubeconfig string
	level      string
	namespaces []string

	client   *kubernetes.Clientset
	warnings *warningsHandler
}

var (
	psaExample = `
	# Pick a location of the kubeconfig file that is not ~/.kube/config.
	%[1]s psa-check --kubeconfig /home/user/Documents/clusters/kubeconfig

	# Check if your namespaces could be upgraded to the restricted level.
	%[1]s psa-check --level restricted
`

	empty       = struct{}{}
	validLevels = map[string]struct{}{
		string(psapi.LevelPrivileged): empty,
		string(psapi.LevelBaseline):   empty,
		string(psapi.LevelRestricted): empty,
	}

	podControllers = map[string]struct{}{
		"Deployment":  empty,
		"DemonSet":    empty,
		"StatefulSet": empty,
		"Job":         empty,
	}
)

// NewCmdPSA creates a cobra.Command that is capable of checking namespaces for{
// for their viability for a given PodSecurity level.
func NewCmdPSA(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := PSAOptions{}

	cmd := cobra.Command{
		Use:     "psa-check",
		Short:   "Verify namespace workloads match the namespace pod security profile",
		Example: fmt.Sprintf(psaExample, parentName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Complete(); err != nil {
				return err
			}

			return o.Run()
		},
	}

	cmd.Flags().StringVarP(&o.kubeconfig, "kubeconfig", "k", "~/.kube/config", "Path to the kubeconfig file to use for PSA check.")
	cmd.Flags().StringVarP(&o.level, "level", "l", "", "The PodSecurity level to check against. The default is the audit level.")
	cmd.Flags().StringSliceVarP(&o.namespaces, "namespaces", "n", []string{}, "The namespaces to check. The default is all namespaces.")

	return &cmd
}

// Validate ensures that all required arguments and flag values are set properly.
func (o *PSAOptions) Validate() error {
	if o.level != "" {
		if _, ok := validLevels[o.level]; !ok {
			return fmt.Errorf("invalid level %q", o.level)
		}
	}

	if o.kubeconfig == "" {
		return fmt.Errorf("kubeconfig must be set")
	}

	return nil
}

// Complete sets all information required for processing the command.
func (o *PSAOptions) Complete() error {
	config, err := clientcmd.BuildConfigFromFlags("", o.kubeconfig)
	if err != nil {
		return err
	}

	// Setup a client with a custom WarningHandler that collects the warnings.
	o.warnings = &warningsHandler{}
	config.WarningHandler = o.warnings
	o.client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

// Run attempts to update the namespace psa enforce label to the psa audit value.
func (o *PSAOptions) Run() error {
	// Get a list of all the namespaces.
	namespaceList, err := o.getNamespaces()
	if err != nil {
		return err
	}

	podSecurityViolations := []*PodSecurityViolation{}
	// Gather all the warnings for each namespace, when enforcing audit-level.
	for _, namespace := range namespaceList.Items {
		psv, err := o.checkNamespacePodSecurity(&namespace)
		if err != nil {
			return err
		}
		if psv == nil {
			continue
		}

		podSecurityViolations = append(podSecurityViolations, psv)

		// Iterate through the pods within a namespace that violate the new
		// PodSecurity level and get the pod's deployment.
		for _, podViolation := range psv.PodViolations {
			klog.V(4).Infof(
				"Pod %q has pod security violations, gathering Pod and Deployemnt Resources",
				podViolation.PodName,
				namespace.Name,
			)

			pod, err := o.client.CoreV1().
				Pods(namespace.Name).
				Get(context.Background(), podViolation.PodName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			podViolation.Pod = pod

			podController, err := o.getPodController(pod)
			if err != nil {
				return err
			}

			podViolation.PodController = podController
		}
	}

	// Print the violations.
	return printViolations(podSecurityViolations)
}

// checkNamespacePodSecurity collects the pod security violations for a given
// namespace on a stricter pod security enforcement.
func (o *PSAOptions) checkNamespacePodSecurity(ns *corev1.Namespace) (*PodSecurityViolation, error) {
	nsCopy := ns.DeepCopy()

	// Get a higher enforcment value.
	targetValue := ""
	switch {
	case o.level != "":
		targetValue = o.level
	case nsCopy.Labels[psapi.AuditLevelLabel] != "":
		targetValue = nsCopy.Labels[psapi.AuditLevelLabel]
	default:
		targetValue = string(psapi.LevelRestricted)
	}

	// Update the pod security enforcement for the dry run.
	nsCopy.Labels[psapi.EnforceLevelLabel] = targetValue

	klog.V(4).Infof("Checking nsCopy %q for violations at level %q", nsCopy.Name, targetValue)

	// Make a server-dry-run update on the nsCopy with the audit-level value.
	_, err := o.client.CoreV1().
		Namespaces().
		Update(
			context.Background(),
			nsCopy,
			metav1.UpdateOptions{DryRun: []string{"All"}},
		)
	if err != nil {
		return nil, err
	}

	// Get the warnings from the server-dry-run update.
	warnings := o.warnings.PopAll()
	if len(warnings) == 0 {
		return nil, nil
	}

	return parseWarnings(warnings), nil
}

// getNamespaces returns the namespace that should be checked for pod security.
// It could be given by the flag. Defaults to all namespaces.
func (o *PSAOptions) getNamespaces() (*corev1.NamespaceList, error) {
	if len(o.namespaces) == 0 {
		namespaceList, err := o.client.CoreV1().
			Namespaces().
			List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		return namespaceList, nil
	}

	// Get the corev1.Namespace representation of the given namespaces.
	// Also validate that those namespaces exist.
	namespaceList := &corev1.NamespaceList{}
	for _, namespace := range o.namespaces {
		ns, err := o.client.CoreV1().
			Namespaces().
			Get(context.Background(), namespace, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		namespaceList.Items = append(namespaceList.Items, *ns)
	}

	return namespaceList, nil
}

// getPodController gets the deployment of a pod.
func (o *PSAOptions) getPodController(pod *corev1.Pod) (any, error) {
	parent := pod.ObjectMeta.OwnerReferences[0]

	// If the pod is owned by a ReplicaSet, get the ReplicaSet's owner.
	if parent.Kind == "ReplicaSet" {
		replicaSet, err := o.client.AppsV1().
			ReplicaSets(pod.Namespace).
			Get(context.Background(), parent.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		parent = replicaSet.ObjectMeta.OwnerReferences[0]
	}

	if _, ok := podControllers[parent.Kind]; !ok {
		return nil, fmt.Errorf("pod controller %q is not supported", parent.Kind)
	}

	// If the pod is owned by a Deployment, get the deployment.
	switch {
	case parent.Kind == "Deployment":
		return o.client.AppsV1().
			Deployments(pod.Namespace).
			Get(context.Background(), parent.Name, metav1.GetOptions{})
	case parent.Kind == "DaemonSet":
		return o.client.AppsV1().
			DaemonSets(pod.Namespace).
			Get(context.Background(), parent.Name, metav1.GetOptions{})
	case parent.Kind == "StatefulSet":
		return o.client.AppsV1().
			StatefulSets(pod.Namespace).
			Get(context.Background(), parent.Name, metav1.GetOptions{})
	case parent.Kind == "Job":
	case parent.Kind == "Job":
		return o.client.BatchV1().
			Jobs(pod.Namespace).
			Get(context.Background(), parent.Name, metav1.GetOptions{})
	}

	klog.Warningf(
		"%s isn't owned by a Deployment: pod.Name=%s, pod.Namespace=%s, pod.OwnerReferences=%v",
		parent.Kind, pod.Name, pod.OwnerReferences, pod.ObjectMeta.OwnerReferences,
	)
	return nil, nil
}

// printViolations prints the PodSecurityViolations as JSON.
func printViolations(podSecurityViolations []*PodSecurityViolation) error {
	return json.NewEncoder(os.Stdout).Encode(podSecurityViolations)
}
