package psa

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	psapi "k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"
)

var (
	psaExample = `
	# Check all namespaces for pod security violations, dynamically applying enforcing-level based on audit- and warn-levels for a server-side dry-run.
	%[1]s psa-check --all-namespaces

	# Check if config's namespace can be upgraded to the 'restricted' security level.
	%[1]s psa-check --level restricted

	# Check if a specific namespace, 'my-namespace', can be upgraded to the 'restricted' security level.
	%[1]s psa-check --level restricted --namespace my-namespace

	# Check if all cluster namespaces can be upgraded to the 'restricted' security level, and print the results as JSON.
	%[1]s psa-check --level restricted --output json

	# Check if all cluster namespaces can be upgraded to the 'restricted' security level, and print the results as YAML.
	%[1]s psa-check --all-namespaces --level restricted --output yaml

	# Check if all cluster namespaces can be upgraded to the 'restricted' security level, and don't return a non-zero exit code if there are violations.
	%[1]s psa-check --all-namespaces --level restricted --quiet
`

	empty       = struct{}{}
	validLevels = map[string]struct{}{
		string(psapi.LevelPrivileged): empty,
		string(psapi.LevelBaseline):   empty,
		string(psapi.LevelRestricted): empty,
	}
)

// PSAOptions contains all the options and configsi for running the PSA command.
type PSAOptions struct {
	quiet bool
	level string

	namespace     string
	allNamespaces bool

	genericclioptions.IOStreams
	printObj    printers.ResourcePrinterFunc
	printFlags  *genericclioptions.PrintFlags
	configFlags *genericclioptions.ConfigFlags
	client      *kubernetes.Clientset
	warnings    *warningsHandler
}

// NewCmdPSA creates a new cobra.Command instance that enables checking
// namespaces for their compatibility with a specified PodSecurity level.
func NewCmdPSA(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := PSAOptions{
		configFlags: &genericclioptions.ConfigFlags{
			Namespace:  utilpointer.String(""),
			KubeConfig: utilpointer.String(""),
		},
		printFlags: genericclioptions.NewPrintFlags("psa").
			WithTypeSetter(scheme.Scheme).
			WithDefaultOutput("json"),
		IOStreams: streams,
	}

	cmd := cobra.Command{
		Use:   "psa-check",
		Short: "Verify namespace workloads match the namespace pod security profile",

		Example: fmt.Sprintf(psaExample, parentName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return fmt.Errorf("completion failed: %w", err)
			}
			if err := o.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			return o.Run()
		},
	}

	fs := cmd.Flags()
	o.configFlags.AddFlags(fs)
	o.printFlags.AddFlags(&cmd)
	fs.StringVar(&o.level, "level", "", "The PodSecurity level to check against.")
	fs.BoolVar(&o.quiet, "quiet", false, "Do not return non-zero exit code on violations.")
	fs.BoolVarP(&o.allNamespaces, "all-namespaces", "A", o.allNamespaces, "If true, check the specified action in all namespaces.")

	return &cmd
}

// Validate ensures that all required arguments and flag values are set properly.
func (o *PSAOptions) Validate() error {
	if o.level == "" {
		return nil
	}

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
		o.printObj = printer.PrintObj

		return nil
	}

	return nil
}
