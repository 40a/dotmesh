package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/lukemarsden/datamesh/cmd/dm/pkg/remotes"
	"github.com/spf13/cobra"
)

func NewCmdPull(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull <remote> <volume> <branch>",
		Short: `Pull new commits from a remote volume to a local copy of that volume`,
		Long: `Pulls commits from a <remote> <volume>'s given <branch> to the
currently active branch of the currently active volume on the currently active
cluster. If <branch> is not specified, try to pull all branches. If <volume> is
not specified, try to pull a volume with the same name on the remote cluster.

Use 'dm clone' to make an initial copy, 'pull' only updates an existing one.

Example: to pull any new commits from the master branch of volume 'postgres' on
cluster 'backups':

    dm pull backups postgres master
`,
		Run: func(cmd *cobra.Command, args []string) {
			err := func() error {
				dm, err := remotes.NewDatameshAPI(configPath)
				if err != nil {
					return err
				}
				// TODO check that filesystem exists on toRemote

				peer, filesystemName, branchName, err := resolveTransferArgs(args)
				if err != nil {
					return err
				}
				transferId, err := dm.RequestTransfer(
					"pull", peer, filesystemName, branchName,
					// (XXX copied from clone.go)
					// 'dm pull' semantics are (for now) always that we pull into the
					// same named filesystem as on the remote, rather than the current
					// filesystem whatever that is.
					filesystemName, branchName,
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
	return cmd
}
