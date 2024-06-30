package main

import (
	"context"
	goflag "flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apiserver/pkg/server"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/stolostron/maestro-addon/test/performance/pkg/hub"
	"github.com/stolostron/maestro-addon/test/performance/pkg/spoke"
)

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(pflag.CommandLine)
	logs.InitLogs()
	defer logs.FlushLogs()

	cmd := &cobra.Command{
		Use:   "maestroperf",
		Short: "Maestro Performance Test Tool",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(newPreparationCommand(), newWatchCommand(), newSpokeCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newPreparationCommand() *cobra.Command {
	o := hub.NewPreparerOptions()
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare clusters or works in Maestro",
		Long:  "Prepare clusters or works in Maestro",
		Run: func(cmd *cobra.Command, args []string) {
			// handle SIGTERM and SIGINT by cancelling the context.
			ctx, cancel := context.WithCancel(context.Background())
			shutdownHandler := server.SetupSignalHandler()
			go func() {
				defer cancel()
				<-shutdownHandler
				klog.Infof("Received SIGTERM or SIGINT signal, shutting down test.")
			}()

			if err := o.Run(ctx); err != nil {
				klog.Errorf("failed to run prepare, %v", err)
			}
		},
	}

	flags := cmd.Flags()
	o.AddFlags(flags)
	return cmd
}

func newWatchCommand() *cobra.Command {
	o := hub.NewPreparerOptions()
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch the works from Maestro",
		Long:  "Watch the works from Maestro",
		Run: func(cmd *cobra.Command, args []string) {
			// handle SIGTERM and SIGINT by cancelling the context.
			ctx, cancel := context.WithCancel(context.Background())
			shutdownHandler := server.SetupSignalHandler()
			go func() {
				defer cancel()
				<-shutdownHandler
				klog.Infof("Received SIGTERM or SIGINT signal, shutting down test.")
			}()

			if err := o.Run(ctx); err != nil {
				klog.Errorf("failed to run watch, %v", err)
			}

			<-ctx.Done()
		},
	}

	flags := cmd.Flags()
	o.AddFlags(flags)
	return cmd
}

func newSpokeCommand() *cobra.Command {
	o := spoke.NewSpokeOptions()
	cmd := &cobra.Command{
		Use:   "spoke",
		Short: "Start agents",
		Long:  "Start agents",
		Run: func(cmd *cobra.Command, args []string) {
			// handle SIGTERM and SIGINT by cancelling the context.
			ctx, cancel := context.WithCancel(context.Background())
			shutdownHandler := server.SetupSignalHandler()
			go func() {
				defer cancel()
				<-shutdownHandler
				klog.Infof("Received SIGTERM or SIGINT signal, shutting down test.")
			}()

			if err := o.Run(ctx); err != nil {
				klog.Errorf("failed to run spoke, %v", err)
			}

			<-ctx.Done()
		},
	}

	flags := cmd.Flags()
	o.AddFlags(flags)
	return cmd
}
