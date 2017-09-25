package commands

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/lukemarsden/datamesh/cmd/dm/pkg/remotes"
	"github.com/spf13/cobra"
)

func NewCmdVolumeSetUpstream(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-upstream",
		Short: "Set or update the default volume on a remote",
		Run: func(cmd *cobra.Command, args []string) {
			err := volumeSetUpstream(cmd, args, out)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}
	return cmd
}

func NewCmdVolumeShow(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display information about a volume",
		Run: func(cmd *cobra.Command, args []string) {
			err := volumeShow(cmd, args, out)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}
	cmd.Flags().BoolVarP(
		&scriptingMode, "scripting", "H", false,
		"scripting mode. Do not print headers, separate fields by "+
			"a single tab instead of arbitrary whitespace.",
	)
	return cmd
}

func NewCmdVolume(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: `Manage volumes`,
		Long: `Manage volumes in the cluster.

Run 'dm volume set-upstream [<volume>] <remote> <remote-volume>' to
change the default remote volume for <volume> on <remote>.

Run 'dm volume show [<volume>]' to show information about the volume.

Where '[<volume>]' is omitted, the current volume (selected by 'dm switch')
is used.`,
	}

	cmd.AddCommand(NewCmdVolumeSetUpstream(os.Stdout))
	cmd.AddCommand(NewCmdVolumeShow(os.Stdout))

	return cmd
}

func volumeSetUpstream(cmd *cobra.Command, args []string, out io.Writer) error {
	dm, err := remotes.NewDatameshAPI(configPath)
	if err != nil {
		return err
	}

	var localVolume, peer, remoteVolume string

	switch len(args) {
	case 2:
		localVolume, err = dm.CurrentVolume()
		if err != nil {
			return err
		}

		peer = args[0]
		remoteVolume = args[1]
	case 3:
		localVolume = args[0]
		peer = args[1]
		remoteVolume = args[2]
	default:
		return fmt.Errorf("Please specify [<volume>] <remote> <remote-volume> as arguments.")
	}

	remote, err := dm.Configuration.GetRemote(peer)
	if err != nil {
		return err
	}

	localNamespace, localVolume, err := remotes.ParseNamespacedVolume(localVolume)
	if err != nil {
		return err
	}

	remoteNamespace, remoteVolume, err := remotes.ParseNamespacedVolumeWithDefault(remoteVolume, remote.User)
	if err != nil {
		return err
	}

	dm.Configuration.SetDefaultRemoteVolumeFor(peer, localNamespace, localVolume, remoteNamespace, remoteVolume)
	return nil
}

func volumeShow(cmd *cobra.Command, args []string, out io.Writer) error {
	dm, err := remotes.NewDatameshAPI(configPath)
	if err != nil {
		return err
	}

	var localVolume string
	if len(args) == 1 {
		localVolume = args[0]
	} else {
		localVolume, err = dm.CurrentVolume()
		if err != nil {
			return err
		}
	}

	namespace, volume, err := remotes.ParseNamespacedVolume(localVolume)
	if err != nil {
		return err
	}
	if scriptingMode {
		fmt.Fprintf(out, "namespace\t%s\n", namespace)
		fmt.Fprintf(out, "name\t%s\n", volume)
	} else {
		fmt.Fprintf(out, "Volume %s/%s:\n", namespace, volume)
	}

	var datameshVolume *remotes.DatameshVolume
	vs, err := dm.AllVolumes()
	if err != nil {
		return err
	}

	for _, v := range vs {
		if v.Name.Namespace == namespace && v.Name.Name == volume {
			datameshVolume = &v
			break
		}
	}
	if datameshVolume == nil {
		return fmt.Errorf("Unable to find volume '%s'", localVolume)
	}

	activeQualified, err := dm.CurrentVolume()
	if err != nil {
		return err
	}
	activeNamespace, activeVolume, err := remotes.ParseNamespacedVolume(activeQualified)
	if err != nil {
		return err
	}

	if namespace == activeNamespace && volume == activeVolume {
		if scriptingMode {
			fmt.Fprintf(out, "active\n")
		} else {
			fmt.Fprintf(out, "Volume is active.\n")
		}
	}

	if scriptingMode {
		fmt.Fprintf(out, "size\t%d\ndirty\t%d\n",
			datameshVolume.SizeBytes,
			datameshVolume.DirtyBytes)
	} else {
		if datameshVolume.DirtyBytes == 0 {
			fmt.Fprintf(out, "Volume size: %s (all clean)\n", prettyPrintSize(datameshVolume.SizeBytes))
		} else {
			fmt.Fprintf(out, "Volume size: %s (%s dirty)\n",
				prettyPrintSize(datameshVolume.SizeBytes),
				prettyPrintSize(datameshVolume.DirtyBytes))
		}
	}

	currentBranch, err := dm.CurrentBranch(localVolume)
	if err != nil {
		return err
	}

	bs, err := dm.AllBranches(localVolume)
	if err != nil {
		return err
	}

	if !scriptingMode {
		fmt.Fprintf(out, "Branches:\n")
	} else {
		fmt.Fprintf(out, "currentBranch\t%s\n", currentBranch)
	}

	for _, branch := range bs {
		if !scriptingMode {
			if branch == currentBranch {
				branch = "* " + branch
			} else {
				branch = "  " + branch
			}
		}

		containerNames := []string{}

		if branch == currentBranch {
			containerInfo, err := dm.RelatedContainers(datameshVolume.Name, branch)
			if err != nil {
				return err
			}
			for _, container := range containerInfo {
				containerNames = append(containerNames, container.Name)
			}
		}

		if scriptingMode {
			fmt.Fprintf(out, "branch\t%s\n", branch)
			for _, c := range containerNames {
				fmt.Fprintf(out, "container\t%s\t%s\n", branch, c)
			}
		} else {
			if len(containerNames) == 0 {
				fmt.Fprintf(out, "%s\n", branch)
			} else {
				fmt.Fprintf(out, "%s (containers: %s)\n", branch, strings.Join(containerNames, ","))
			}
		}
	}

	remotes := dm.Configuration.GetRemotes()
	keys := []string{}
	// sort the keys so we can iterate over in human friendly order
	for k, _ := range remotes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		remoteNamespace, remoteVolume, ok := dm.Configuration.DefaultRemoteVolumeFor(k, namespace, volume)
		if ok {
			if scriptingMode {
				fmt.Fprintf(out, "defaultRemoteVolume\t%s\t%s/%s\n",
					k,
					remoteNamespace,
					remoteVolume)
			} else {
				fmt.Fprintf(out, "Tracks volume %s/%s on remote %s\n", remoteNamespace, remoteVolume, k)
			}
		}
	}
	return nil
}
