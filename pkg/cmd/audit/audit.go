package audit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/openshift/cluster-debug-tools/pkg/util"
)

var (
	auditExample = `
	# find all GC calls to deployments in any apigroup (extensions or apps)
	%[1]s audit -f audit.log --user=system:serviceaccount:kube-system:generic-garbage-collector --resource=deployments.*

	# find all failed calls to kube-system and olm namespaces
	%[1]s audit -f audit.log --namespace=kube-system --namespace=openshift-operator-lifecycle-manager --failed-only

	# find all GETs against deployments and any resource under config.openshift.io
	%[1]s audit -f audit.log --resource=deployments.* --resource=*.config.openshift.io --verb=get

	# find CREATEs of everything except SAR and tokenreview
	%[1]s audit -f audit.log --verb=create --resource=*.* --resource=-subjectaccessreviews.* --resource=-tokenreviews.*

	# filter event by stages
	%[1]s audit -f audit.log --verb=get --stage=ResponseComplete --output=top --by=verb
`
)

type AuditOptions struct {
	fileWriter *util.MultiSourceFileWriter
	builder    *resource.Builder
	args       []string

	verbs        []string
	resources    []string
	subresources []string
	namespaces   []string
	names        []string
	users        []string
	uids         []string
	filenames    []string
	failedOnly   bool
	output       string
	topBy        string
	beforeString string
	afterString  string
	stages       []string

	genericclioptions.IOStreams
}

func NewAuditOptions(streams genericclioptions.IOStreams) *AuditOptions {
	return &AuditOptions{
		IOStreams: streams,
		stages: []string{
			// We are making RequestReceived the default stage,
			// this will provide a protection against double counting of events.
			"ResponseComplete",
		},
	}
}

func NewCmdAudit(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAuditOptions(streams)

	cmd := &cobra.Command{
		Use:          "audit -f=audit.file [flags]",
		Short:        "Inspects the audit logs captured during CI test run.",
		Example:      fmt.Sprintf(auditExample, parentName),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&o.filenames, "filename", "f", o.filenames, "Search for audit logs that contains specified URI")
	cmd.Flags().StringVarP(&o.output, "output", "o", o.output, "Choose your output format")
	cmd.Flags().StringSliceVar(&o.uids, "uid", o.uids, "Only match specific UIDs")
	cmd.Flags().StringSliceVar(&o.verbs, "verb", o.verbs, "Filter result of search to only contain the specified verb (eg. 'update', 'get', etc..)")
	cmd.Flags().StringSliceVar(&o.resources, "resource", o.resources, "Filter result of search to only contain the specified resource.)")
	cmd.Flags().StringSliceVar(&o.subresources, "subresource", o.subresources, "Filter result of search to only contain the specified subresources.  \"-*\" means no subresource)")
	cmd.Flags().StringSliceVarP(&o.namespaces, "namespace", "n", o.namespaces, "Filter result of search to only contain the specified namespace.)")
	cmd.Flags().StringSliceVar(&o.names, "name", o.names, "Filter result of search to only contain the specified name.)")
	cmd.Flags().StringSliceVar(&o.users, "user", o.users, "Filter result of search to only contain the specified user.)")
	cmd.Flags().StringVar(&o.topBy, "by", o.topBy, "Switch the top output format (eg. -o top -by [verb,user,resource,httpstatus,namespace]).")
	cmd.Flags().BoolVar(&o.failedOnly, "failed-only", false, "Filter result of search to only contain http failures.)")
	cmd.Flags().StringVar(&o.beforeString, "before", o.beforeString, "Filter result of search to only before a timestamp.)")
	cmd.Flags().StringVar(&o.afterString, "after", o.afterString, "Filter result of search to only after a timestamp.)")
	cmd.Flags().StringSliceVarP(&o.stages, "stage", "s", o.stages, "Filter result by event stage (eg. 'RequestReceived', 'ResponseComplete'), if omitted all stages will be included)")

	return cmd
}

func (o *AuditOptions) Complete(command *cobra.Command, args []string) error {
	return nil
}

func (o *AuditOptions) Validate() error {
	switch {
	case o.output == "":
	case strings.HasPrefix(o.output, "top"):
		_, err := topN(o.output)
		if err != nil {
			return err
		}
		if err := validateTopBy(o.topBy); err != nil {
			return nil
		}
	case o.output == "wide":
	case o.output == "json":
	default:
		return fmt.Errorf("unsupported output format: top=N, wide, json")
	}

	return nil
}

func validateTopBy(topBy string) error {
	switch topBy {
	case "verb":
	case "user":
	case "resource":
	case "httpstatus":
	case "namespace":
	default:
		return fmt.Errorf("unsupported -by value: [verb,user,resource,httpstatus,namespace]")
	}
	return nil
}

func topN(output string) (int, error) {
	if output == "top" {
		return 10, nil
	}
	if !strings.HasPrefix(output, "top=") {
		return 10, fmt.Errorf("%q is not top=N", output)
	}

	nString := output[len("top="):]
	n, err := strconv.ParseInt(nString, 10, 32)
	if err != nil {
		return 10, err
	}
	return int(n), nil
}

func (o *AuditOptions) Run() error {
	filters := AuditFilters{}
	if len(o.uids) > 0 {
		filters = append(filters, &FilterByUIDs{UIDs: sets.NewString(o.uids...)})
	}
	if len(o.names) > 0 {
		filters = append(filters, &FilterByNames{Names: sets.NewString(o.names...)})
	}
	if len(o.namespaces) > 0 {
		filters = append(filters, &FilterByNamespaces{Namespaces: sets.NewString(o.namespaces...)})
	}
	if len(o.stages) > 0 {
		filters = append(filters, &FilterByStage{Stages: sets.NewString(o.stages...)})
	}
	if len(o.beforeString) > 0 {
		t, err := time.Parse(time.RFC3339, o.beforeString)
		if err != nil {
			return err
		}
		filters = append(filters, &FilterByBefore{Before: t})
	}
	if len(o.afterString) > 0 {
		t, err := time.Parse(time.RFC3339, o.afterString)
		if err != nil {
			return err
		}
		filters = append(filters, &FilterByAfter{After: t})
	}
	if len(o.resources) > 0 {
		resources := map[schema.GroupResource]bool{}
		for _, resource := range o.resources {
			parts := strings.Split(resource, ".")
			gr := schema.GroupResource{}
			gr.Resource = parts[0]
			if len(parts) >= 2 {
				gr.Group = strings.Join(parts[1:], ".")
			}
			resources[gr] = true
		}

		filters = append(filters, &FilterByResources{Resources: resources})
	}
	if len(o.subresources) > 0 {
		filters = append(filters, &FilterBySubresources{Subresources: sets.NewString(o.subresources...)})
	}
	if len(o.users) > 0 {
		filters = append(filters, &FilterByUser{Users: sets.NewString(o.users...)})
	}
	if len(o.verbs) > 0 {
		filters = append(filters, &FilterByVerbs{Verbs: sets.NewString(o.verbs...)})
	}
	if o.failedOnly {
		filters = append(filters, &FilterByFailures{})
	}

	events, err := GetEvents(o.filenames...)
	if err != nil {
		return err
	}
	events = filters.FilterEvents(events...)
	switch {
	case o.output == "":
		PrintAuditEvents(o.Out, events)
	case strings.HasPrefix(o.output, "top"):
		numToDisplay, err := topN(o.output)
		if err != nil {
			return err
		}
		PrintSummary(o.Out, events)
		switch o.topBy {
		case "verb":
			PrintTopByVerbAuditEvents(o.Out, numToDisplay, events)
		case "user":
			PrintTopByUserAuditEvents(o.Out, numToDisplay, events)
		case "resource":
			PrintTopByResourceAuditEvents(o.Out, numToDisplay, events)
		case "httpstatus":
			PrintTopByHTTPStatusCodeAuditEvents(o.Out, numToDisplay, events)
		case "namespace":
			PrintTopByNamespace(o.Out, numToDisplay, events)
		default:
			return fmt.Errorf("unsupported -by value")
		}
	case o.output == "wide":
		PrintAuditEventsWide(o.Out, events)
	case o.output == "json":
		encoder := json.NewEncoder(o.Out)
		for _, event := range events {
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported output format")
	}

	return nil
}
