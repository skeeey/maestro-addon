package main

import (
	goflag "flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"

	"github.com/stolostron/maestro-addon/pkg/cmd/hub"
	"github.com/stolostron/maestro-addon/pkg/version"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func execute() error {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(pflag.CommandLine)
	logs.InitLogs()
	defer logs.FlushLogs()

	command := newCommand()
	return command.Execute()
}

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "maestroaddon",
		Short: "Maestro AddOn",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}

	if v := version.Get().String(); len(v) == 0 {
		cmd.Version = "<unknown>"
	} else {
		cmd.Version = v
	}

	cmd.AddCommand(hub.NewHubManager())
	return cmd
}
