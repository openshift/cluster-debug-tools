package psa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	psapi "k8s.io/pod-security-admission/api"

	"github.com/spf13/cobra"
)

// PSAOptions contains all the options and configsi for running the PSA command.
type PSAOptions struct {
	kubeconfig string
	level      string

	client   *kubernetes.Clientset
	warnings *warningsHandler
}

var (
	psaExample = `
	# pick a location of the kubeconfig file that is not ~/.kube/config.
	%[1]s psa-check --kubeconfig /home/user/Documents/clusters/kubeconfig

	# pick a pod-security.kubernetes.io/enforce: restricted.
	%[1]s psa-check --level restricted
`

	anyLevel    = struct{}{}
	validLevels = map[string]struct{}{
		string(psapi.LevelPrivileged): anyLevel,
		string(psapi.LevelBaseline):   anyLevel,
		string(psapi.LevelRestricted): anyLevel,
	}
)

// NewCmdPSA creates a cobra.Command that is hooked up to the kubectl-dev_tool.
func NewCmdPSA(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := PSAOptions{}

	cmd := cobra.Command{
		Use:     "psa-check",
		Short:   "Check namespaces for a higher pod security enforce value.",
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

	cmd.Flags().StringVar(&o.kubeconfig, "kubeconfig", "~/.kube/config", "Path to the kubeconfig file to use for PSA check.")
	cmd.Flags().StringVar(&o.level, "level", "", "The PodSecurity level to check against. The default is the audit level.")

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
	namespaceList, err := o.client.CoreV1().
		Namespaces().
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	podSecurityViolations := []*PodSecurityViolation{}
	// Gather all the warnings for each namespace, when enforcing audit-level.
	for _, namespace := range namespaceList.Items {
		psv, err := o.createPodSecurityViolation(&namespace)
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

			deployment, err := o.getDeployment(pod)
			if err != nil {
				return err
			}
			podViolation.Deployment = deployment
		}
	}

	// Print the violations.
	return printViolations(podSecurityViolations)
}

// createPodSecurityViolation collects the pod security violations for a given
// namespace on a stricter pod security enforcement.
func (o *PSAOptions) createPodSecurityViolation(ns *corev1.Namespace) (*PodSecurityViolation, error) {
	// If the namespace is already enforcing restricted, there shouldn't be any violations.
	if ns.Labels[psapi.EnforceLevelLabel] == string(psapi.LevelRestricted) {
		return nil, nil
	}

	namespace := ns.DeepCopy()

	// Get a higher enforcment value.
	targetValue := ""
	switch {
	case o.level != "":
		targetValue = o.level
	case namespace.Labels[psapi.AuditLevelLabel] != "":
		targetValue = namespace.Labels[psapi.AuditLevelLabel]
	default:
		targetValue = string(psapi.LevelRestricted)
	}

	// Update the pod security enforcement for the dry run.
	namespace.Labels[psapi.EnforceLevelLabel] = targetValue

	klog.V(4).Infof("Checking namespace %q for violations at level %q", namespace.Name, targetValue)

	// Make a server-dry-run update on the namespace with the audit-level value.
	_, err := o.client.CoreV1().
		Namespaces().
		Update(
			context.Background(),
			namespace,
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

// getDeployment gets the deployment of a pod.
func (o *PSAOptions) getDeployment(pod *corev1.Pod) (*appsv1.Deployment, error) {
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

	// If the pod is owned by a Deployment, get the deployment.
	if parent.Kind != "Deployment" {
		klog.Warningf(
			"%s isn't owned by a Deployment: pod.Name=%s, pod.Namespace=%s, pod.OwnerReferences=%v",
			parent.Kind, pod.Name, pod.OwnerReferences, pod.ObjectMeta.OwnerReferences,
		)
		return nil, nil
	}

	deployment, err := o.client.AppsV1().
		Deployments(pod.Namespace).
		Get(context.Background(), parent.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

// printViolations prints the PodSecurityViolations as JSON.
func printViolations(podSecurityViolations []*PodSecurityViolation) error {
	return json.NewEncoder(os.Stdout).Encode(podSecurityViolations)
}
