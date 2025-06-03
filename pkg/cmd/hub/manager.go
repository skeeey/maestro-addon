package hub

import (
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"k8s.io/utils/clock"

	"github.com/stolostron/maestro-addon/pkg/hub"
	"github.com/stolostron/maestro-addon/pkg/version"
)

// NewHubManager generates a command to start hub manager
func NewHubManager() *cobra.Command {
	o := hub.NewMaestroAddOnManagerOptions()
	cmdConfig := controllercmd.
		NewControllerCommandConfig("maestro-addon-manager", version.Get(), o.RunHubManager, clock.RealClock{})
	cmd := cmdConfig.NewCommand()
	cmd.Use = "manager"
	cmd.Short = "Start the Maestro AddOn Hub Manager"

	flags := cmd.Flags()
	o.AddFlags(flags)
	flags.BoolVar(&cmdConfig.DisableLeaderElection, "disable-leader-election", false,
		"Disable leader election for the manager.")

	return cmd
}
