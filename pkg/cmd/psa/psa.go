package psa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	psapi "k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	"github.com/spf13/cobra"
)

// PSAOptions contains all the options and configsi for running the PSA command.
type PSAOptions struct {
	quiet bool
	level string

	namespace     string
	allNamespaces bool

	printObj    printers.ResourcePrinterFunc
	printFlags  *genericclioptions.PrintFlags
	configFlags *genericclioptions.ConfigFlags
	client      *kubernetes.Clientset
	warnings    *warningsHandler
}

var (
	psaExample = `
	# Check if all cluster namespaces can be upgraded to the 'restricted' security level.
	%[1]s psa-check --level restricted

	# Check if a specific namespace, 'my-namespace', can be upgraded to the 'restricted' security level.
	%[1]s psa-check --level restricted --namespace my-namespace
`

	empty       = struct{}{}
	validLevels = map[string]struct{}{
		string(psapi.LevelPrivileged): empty,
		string(psapi.LevelBaseline):   empty,
		string(psapi.LevelRestricted): empty,
	}

	podControllers = map[string]struct{}{
		"Deployment":  empty,
		"DaemonSet":   empty,
		"StatefulSet": empty,
		"CronJob":     empty,
		"Job":         empty,
	}
)

// NewCmdPSA creates a new cobra.Command instance that enables checking
// namespaces for their compatibility with a specified PodSecurity level.
func NewCmdPSA(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := PSAOptions{
		configFlags: &genericclioptions.ConfigFlags{
			Namespace:  utilpointer.StringPtr(""),
			KubeConfig: utilpointer.StringPtr(""),
		},
		printFlags: genericclioptions.NewPrintFlags("psa").WithTypeSetter(scheme.Scheme),
	}

	cmd := cobra.Command{
		Use:   "psa-check",
		Short: "Verify namespace workloads match the namespace pod security profile",

		Example: fmt.Sprintf(psaExample, parentName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			if err := o.Complete(); err != nil {
				return fmt.Errorf("completion failed: %w", err)
			}

			return o.Run()
		},
	}

	fs := cmd.Flags()
	o.configFlags.AddFlags(fs)
	o.printFlags.AddFlags(&cmd)
	fs.StringVar(&o.level, "level", "restricted", "The PodSecurity level to check against.")
	fs.BoolVar(&o.quiet, "quiet", false, "Do not return non-zero exit code on violations.")
	fs.BoolVarP(&o.allNamespaces, "all-namespaces", "A", o.allNamespaces, "If true, check the specified action in all namespaces.")

	return &cmd
}

// Validate ensures that all required arguments and flag values are set properly.
func (o *PSAOptions) Validate() error {
	if _, ok := validLevels[o.level]; !ok {
		return fmt.Errorf("invalid level %q", o.level)
	}
	return nil
}

// Complete sets all information required for processing the command.
func (o *PSAOptions) Complete() error {
	config := o.configFlags.ToRawKubeConfigLoader()
	restConfig, err := config.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create rest config: %w", err)
	}

	namespace, _, err := config.Namespace()
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}
	o.namespace = namespace

	// Setup a client with a custom WarningHandler that collects the warnings.
	o.warnings = &warningsHandler{}
	restConfig.WarningHandler = o.warnings
	o.client, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if o.printFlags.OutputFormat != nil && len(*o.printFlags.OutputFormat) > 0 {
		printer, err := o.printFlags.ToPrinter()
		if err != nil {
			return err
		}
		o.printObj = func(object runtime.Object, writer io.Writer) error {
			return printer.PrintObj(object, writer)
		}

		return nil
	}

	o.printObj = func(object runtime.Object, writer io.Writer) error {
		if err := json.NewEncoder(writer).Encode(object); err != nil {
			return err
		}

		return nil
	}

	return nil
}

// Run attempts to update the namespace psa enforce label to the psa audit value.
func (o *PSAOptions) Run() error {
	// Get a list of all the namespaces.
	namespaceList, err := o.getNamespaces()
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
	}

	podSecurityViolations := []*PodSecurityViolation{}
	// Gather all the warnings for each namespace, when enforcing audit-level.
	for _, namespace := range namespaceList.Items {
		psv, err := o.checkNamespacePodSecurity(&namespace)
		if err != nil {
			return fmt.Errorf("failed to check namespace %q: %w", namespace.Name, err)
		}
		if psv == nil {
			continue
		}

		klog.V(4).Infof(
			"Pod %q has pod security violations, gathering Pod and Deployment Resources",
			psv.PodName,
			namespace.Name,
		)

		psv.Pod, err = o.client.CoreV1().
			Pods(namespace.Name).
			Get(context.Background(), psv.PodName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf(
				"failed to get pod %q from %q, which violates %q: %w",
				psv.PodName,
				namespace.Name,
				psv.Violations[0],
				err,
			)
		}

		psv.PodControllers, err = o.getPodControllers(psv.Pod)
		if err != nil {
			return fmt.Errorf("failed to get pod controller: %w", err)
		}

		podSecurityViolations = append(podSecurityViolations, psv)
	}

	if len(podSecurityViolations) == 0 {
		return nil
	}

	// Print the violations.
	w := printers.GetNewTabWriter(os.Stdout)
	defer w.Flush()

	if err := o.printObj(podSecurityViolations, w); err != nil {
		return fmt.Errorf("failed to print pod security violations: %w", err)
	}

	if o.quiet {
		return nil
	}

	return fmt.Errorf("found %d pod security violations", len(podSecurityViolations))
}

// checkNamespacePodSecurity collects the pod security violations for a given
// namespace on a stricter pod security enforcement.
func (o *PSAOptions) checkNamespacePodSecurity(ns *corev1.Namespace) (*PodSecurityViolation, error) {
	nsCopy := ns.DeepCopy()

	// Update the pod security enforcement for the dry run.
	nsCopy.Labels[psapi.EnforceLevelLabel] = o.level

	klog.V(4).Infof("Checking nsCopy %q for violations at level %q", nsCopy.Name, o.level)

	// Make a server-dry-run update on the nsCopy with the audit-level value.
	_, err := o.client.CoreV1().
		Namespaces().
		Update(
			context.Background(),
			nsCopy,
			metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}},
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
	if o.allNamespaces {
		namespaceList, err := o.client.CoreV1().
			Namespaces().
			List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list namespaces: %w", err)
		}

		return namespaceList, nil
	}

	// Get the corev1.Namespace representation of the given namespaces.
	// Also validate that those namespaces exist.
	ns, err := o.client.CoreV1().
		Namespaces().
		Get(context.Background(), o.namespace, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace %q: %w", o.namespace, err)
	}

	return &corev1.NamespaceList{
		Items: []corev1.Namespace{*ns},
	}, nil
}

// getPodControllers gets the deployment of a pod.
func (o *PSAOptions) getPodControllers(pod *corev1.Pod) ([]any, error) {
	if len(pod.ObjectMeta.OwnerReferences) == 0 {
		return nil, nil
	}

	podControllers := []any{}
	for _, parent := range pod.ObjectMeta.OwnerReferences {
		any, err := o.getPodController(pod, parent)
		if err != nil {
			return nil, fmt.Errorf("failed to get pod controller: %w", err)
		}
		if any != nil {
			podControllers = append(podControllers, any)
		}
	}

	return podControllers, nil
}

// getPodController gets the deployment of a pod.
func (o *PSAOptions) getPodController(pod *corev1.Pod, parent metav1.OwnerReference) (any, error) {
	// If the pod is owned by a ReplicaSet, get the ReplicaSet's owner.
	if parent.Kind == "ReplicaSet" {
		replicaSet, err := o.client.AppsV1().
			ReplicaSets(pod.Namespace).
			Get(context.Background(), parent.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get ReplicaSet %q: %w", parent.Name, err)
		}

		if len(replicaSet.ObjectMeta.OwnerReferences) == 0 {
			return nil, nil
		}

		parent = replicaSet.ObjectMeta.OwnerReferences[0]
	}

	if _, ok := podControllers[parent.Kind]; !ok {
		return nil, nil
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
	case parent.Kind == "CronJob":
	case parent.Kind == "Job":
		return o.client.BatchV1().
			Jobs(pod.Namespace).
			Get(context.Background(), parent.Name, metav1.GetOptions{})
	}

	klog.Warningf(
		"%s isn't owned by a known pod controller: pod.Name=%s, pod.Namespace=%s, pod.OwnerReferences=%v",
		parent.Kind, pod.Name, pod.OwnerReferences, pod.ObjectMeta.OwnerReferences,
	)

	return nil, nil
}
