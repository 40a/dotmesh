package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/lukemarsden/datamesh/cmd/dm/pkg/remotes"
	"github.com/spf13/cobra"
)

var cloneLocalVolume string

func NewCmdClone(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <remote> <volume> <branch>",
		Short: `Make a complete copy of a remote volume`,
		// XXX should this specify a branch?
		Long: `Make a complete copy on the current active cluster of the given
<branch> of the given <volume> on the given <remote>. By default, name the
volume the same here as it's named there.

Example: to clone the 'repro_bug_1131' branch from volume 'billing_postgres' on
cluster 'devdata' to your currently active local datamesh instance which has no
copy of 'app_billing_postgres' at all yet:

    dm clone devdata billing_postgres repro_bug_1131
`,
		Run: func(cmd *cobra.Command, args []string) {
			err := func() error {
				dm, err := remotes.NewDatameshAPI(configPath)
				if err != nil {
					return err
				}
				// TODO check that filesystem does _not_ exist on toRemote

				peer, filesystemName, branchName, err := resolveTransferArgs(args)
				if err != nil {
					return err
				}
				transferId, err := dm.RequestTransfer(
					"pull", peer,
					cloneLocalVolume, branchName,
					filesystemName, branchName,
					// TODO also switch to the remote?
				)
				if err != nil {
					return err
				}
				err = dm.PollTransfer(transferId, out)
				if err != nil {
					return err
				}

				return nil
			}()
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}

	cmd.PersistentFlags().StringVarP(&cloneLocalVolume, "local-volume", "", "",
		"Local volume name to create")

	return cmd
}
