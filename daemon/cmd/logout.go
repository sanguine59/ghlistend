package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sanguine59/ghlistend/daemon/internal/auth"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the stored GitHub token from the OS keyring",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	if err := auth.DeleteToken(); err != nil {
		if errors.Is(err, auth.ErrNoToken) {
			fmt.Println("no token to remove")
			return nil
		}
		return err
	}
	fmt.Println("token removed")
	return nil
}
