package certs

import (
	"fmt"
	"github.com/openshift/cluster-debug-tools/pkg/cmd/certinspection"
	"github.com/openshift/cluster-debug-tools/pkg/cmd/locateinclustercerts"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewCmdCerts(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "certs",
		Short:        "Helpers for certs",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return fmt.Errorf("use subcommands")
		},
	}

	cmd.AddCommand(certinspection.NewCmdCertInspection(streams))
	cmd.AddCommand(locateinclustercerts.NewCmdLocateInClusterCerts(streams))

	return cmd
}
