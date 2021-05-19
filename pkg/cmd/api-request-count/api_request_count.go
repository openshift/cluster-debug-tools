package api_request_count

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	apiv1 "github.com/openshift/api/apiserver/v1"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

var (
	example = `
    # counts resources used by users and generates a static HTML page.
    #
    # "bob": {
    #  "configmaps":1,
    #  "secrets":2,
    # }
    %[1]s apicount -f path/to/apirequestcounts --tmpl path/to/html/template

	# counts users of a resource and generates a static HTML page.
    #
    # "secrets": {
    #   "bob":1,
    #   "alice":2,
    # }
	%[1]s apicount --by resource -f path/to/apirequestcounts --tmpl path/to/html/template
`
)

func NewCmdAPIRequestCount(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := apiRequestCountOptions{}

	cmd := &cobra.Command{
		Use:          "apicount [flags]",
		Short:        "Creates a static HTML page for api requests",
		Example:      fmt.Sprintf(example, parentName),
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

	cmd.Flags().StringVarP(&o.inputDirectory, "datadir", "f", o.inputDirectory, "A directory which contains api requests data")
	cmd.Flags().StringVarP(&o.outputDirectory, "outdir", "o", o.outputDirectory, "The path of the output directory")
	cmd.Flags().StringVarP(&o.templateDirectory, "tmpl", "t", o.templateDirectory, "The path of the HTML template directory")
	cmd.Flags().StringVar(&o.by, "by", o.by, "Specifies a aggregateFn to apply over the original data (eg. -by [user,resource])")
	cmd.Flags().StringSliceVar(&o.verbs, "verb", o.verbs, "Filter result of search to only contain the specified verb (eg. 'update', 'get', etc..)")

	return cmd
}

type apiRequestCountOptions struct {
	inputDirectory    string
	outputDirectory   string
	templateDirectory string

	by    string
	verbs []string

	cwd string
}

func (o *apiRequestCountOptions) Complete(command *cobra.Command, args []string) error {
	currentWorkingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	o.cwd = currentWorkingDir

	// apply default values
	if len(o.inputDirectory) == 0 {
		o.inputDirectory = filepath.Join(o.cwd, "apirequestcounts")
	}
	if len(o.outputDirectory) == 0 {
		o.outputDirectory = path.Join(o.cwd, "api-request-count-dashboard")
	}
	if len(o.by) == 0 {
		o.by = "user"
	}

	return nil
}

func (o *apiRequestCountOptions) Validate() error {
	if o.by != "resource" && o.by != "user" {
		return fmt.Errorf("incorrect output ordering was specified %q", o.by)
	}
	if len(o.templateDirectory) == 0 {
		return errors.New("a path to the HTML template directory is mandatory")
	}
	if _, err := os.Stat(path.Join(o.templateDirectory, "index.html")); err != nil {
		return fmt.Errorf("make sure that the template \"index.html\" fle exists in %s, got err = %v", o.templateDirectory, err)
	}
	return nil
}

func (o *apiRequestCountOptions) Run() error {
	res := map[string]map[string]int64{}
	var groupBy aggregateFn

	switch o.by {
	case "resource":
		groupBy = groupByResource
	case "user":
		groupBy = groupByUser
	}

	var filters apiRequestFilters
	if len(o.verbs) > 0 {
		filters = append(filters, filterByVerbs(o.verbs))
	}

	klog.Infof("starting processing data from %s", o.inputDirectory)
	if err := walkData(o.inputDirectory, func(unprocessedAPIRequests *apiv1.APIRequestCount) error {
		apiRequests := getRequestHistoryForTheLast(0, 0, true, unprocessedAPIRequests.Status)
		apiRequests = filters.apply(apiRequests)
		mergeMaps(res, groupBy(unprocessedAPIRequests.Name, apiRequests))
		return nil
	}); err != nil {
		return fmt.Errorf("failed while processing data, err = %v", err)
	}

	klog.Infof("creating a new dashboard at %s", o.outputDirectory)
	klog.Info("serializing data")
	rawData, err := serializeDataWithWriteOrder(res, primaryKeyOrder(res), secondaryKeyOrder)
	if err != nil {
		return fmt.Errorf("failed to serialized data to a JSON file, err %v", err)
	}
	klog.Info("crating directory structure")
	if err := copyDir(o.templateDirectory, o.outputDirectory); err != nil {
		return fmt.Errorf("failed to copy the directory tree from %q to %q, due to err = %v", o.templateDirectory, o.outputDirectory, err)
	}
	klog.Info("saving the dashboard")
	return serializeToHTMLTemplate(rawData, filepath.Join(o.templateDirectory, "index.html"), path.Join(o.outputDirectory, "index.html"))
}
