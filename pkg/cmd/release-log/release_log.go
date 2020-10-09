package release_log

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	releaseLogExample = `
	# print all commits between specified time range
	%[1]s release-log quay.io/openshift-release-dev/ocp-release:4.6.0-rc.1-x86_64 --since=YYYY-MM-DD --until=YYYY-MM-DD
`
)

type ReleaseLogOptions struct {
	configFlags  *genericclioptions.ConfigFlags
	builderFlags *genericclioptions.ResourceBuilderFlags

	since        string
	until        string
	releaseImage string

	genericclioptions.IOStreams
}

func NewReleaseLogOptions(streams genericclioptions.IOStreams) *ReleaseLogOptions {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.Namespace = nil

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	return &ReleaseLogOptions{
		configFlags: configFlags,
		builderFlags: genericclioptions.NewResourceBuilderFlags().
			WithLocal(true).WithScheme(scheme).WithAllNamespaces(true).WithLatest().WithAll(true),

		IOStreams: streams,
	}
}

func NewCmdReleaseLog(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewReleaseLogOptions(streams)

	cmd := &cobra.Command{
		Use:          "release-log <release-image> [flags]",
		Short:        "List release commits between time range.",
		Example:      fmt.Sprintf(releaseLogExample, parentName),
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

	cmd.Flags().StringVarP(&o.since, "since", "", o.since, "Only commits after this date will be returned")
	cmd.Flags().StringVarP(&o.until, "until", "", o.until, "Only commits before this date will be returned")

	o.configFlags.AddFlags(cmd.Flags())
	o.builderFlags.AddFlags(cmd.Flags())

	return cmd
}

func (o *ReleaseLogOptions) Complete(command *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.releaseImage = args[0]
	}
	if len(o.since) == 0 {
		return fmt.Errorf("since must be set to valid time YYYY-MM-DD")
	}
	return nil
}

func (o *ReleaseLogOptions) Validate() error {
	if len(o.releaseImage) == 0 {
		return fmt.Errorf("release image must be specified (eg. quay.io/openshift-release-dev/ocp-release:4.6.0-rc.1-x86_64)")
	}
	return nil
}

func (o *ReleaseLogOptions) Run() error {
	sinceTime, err := time.Parse("2006-01-02", o.since)
	if err != nil {
		return err
	}

	var untilTime time.Time
	if len(o.until) == 0 {
		untilTime = time.Now()
	} else {
		untilTime, err = time.Parse("2006-01-02", o.until)
		if err != nil {
			return err
		}
	}

	payloadRepositories, err := getReleaseImageRepositories(o.releaseImage)
	if err != nil {
		return err
	}

	for _, r := range payloadRepositories {
		parts := strings.Split(strings.TrimPrefix(r, "https://github.com/"), "/")
		commits, err := listRepositoryCommits(parts[0], parts[1], sinceTime, untilTime)
		if err != nil {
			return err
		}
		for _, c := range commits {
			if strings.Contains(c.Commit.GetMessage(), "Merge pull request") {
				continue
			}
			messageLines := strings.Split(strings.TrimSpace(c.Commit.GetMessage()), "\n")
			mergeTime := c.GetCommit().GetCommitter().GetDate().Format("2006-01-02")

			fmt.Printf("* %s/%s: [%s] %s\n%s\n\n", parts[0], parts[1], mergeTime, c.GetHTMLURL(), messageLines[0])
		}
	}
	return nil
}
