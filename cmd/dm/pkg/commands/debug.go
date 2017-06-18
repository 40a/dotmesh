package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lukemarsden/datamesh/cmd/dm/pkg/remotes"
	"github.com/spf13/cobra"
)

func NewCmdDebug(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Make API calls",
		Run: func(cmd *cobra.Command, args []string) {
			err := func() error {
				method := args[0]
				dm, err := remotes.NewDatameshAPI(configPath)
				if err != nil {
					return err
				}
				var response interface{}
				err = dm.CallRemote(context.Background(), method, nil, &response)
				r, err := json.Marshal(response)
				if err != nil {
					return err
				}
				fmt.Printf(string(r) + "\n")
				return err
			}()
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}
	return cmd
}
