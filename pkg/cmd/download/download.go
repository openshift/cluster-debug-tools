package download

import (
	"fmt"
	"path"
	"regexp"
	"time"

	"github.com/openshift/cluster-debug-tools/pkg/tools"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const downloadExample = `
	%[1]s download [url] --regex="gather-extra\/artifacts\/.*.json"
`

type DownloadOptions struct {
	gcsClient   *tools.GCSClient
	name        string
	artifactURL string
	saveDir     string
	filter      string
	filterReg   *regexp.Regexp

	genericclioptions.IOStreams
}

func (o *DownloadOptions) Complete(command *cobra.Command, args []string) error {
	switch {
	case len(args) < 1:
		return fmt.Errorf("must provide artifact url as first argument")
	default:
		o.artifactURL = args[0]
		if o.saveDir == "" {
			o.saveDir = path.Base(o.artifactURL)
		}
	}
	return nil
}

func (o *DownloadOptions) Validate() error {
	if !tools.IsGCSLink(o.artifactURL) {
		return fmt.Errorf("url is not a Prow or GCS Link: %s", o.artifactURL)
	}
	if r, err := regexp.Compile(o.filter); err != nil {
		return err
	} else {
		o.filterReg = r
	}
	return nil
}

func (o *DownloadOptions) Run() error {
	now := time.Now()
	fmt.Printf("Downloading and saving files directory (%s)...\n", o.saveDir)
	if err := o.gcsClient.FetchAndSaveArtifacts(o.artifactURL, o.saveDir, o.filterReg); err != nil {
		return fmt.Errorf("error fetching and saving: %s", err)
	}
	fmt.Printf("Done downloading and saving (took %s)\n", time.Since(now).Round(time.Millisecond))
	return nil
}

func NewDownloadOptions(parentName string, streams genericclioptions.IOStreams) *DownloadOptions {
	return &DownloadOptions{
		name:      parentName,
		gcsClient: tools.NewGCSClient(),
		IOStreams: streams,
	}
}

func NewDownloadCommand(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDownloadOptions(parentName, streams)
	cmd := &cobra.Command{
		Use:          "download [url]",
		Short:        fmt.Sprintf("Download %s files from GCS based off of a regex for file paths", parentName),
		Example:      fmt.Sprintf(downloadExample, parentName),
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
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

	cmd.Flags().StringVarP(&o.filter, "regex", "r", o.filter, "regex filter to download files, if not provided it will match all")
	cmd.Flags().StringVarP(&o.saveDir, "dir", "d", o.saveDir, "download dir, if not provided will be prowjob run id ")
	return cmd
}
