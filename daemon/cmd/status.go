package cmd

import (
	"context"
	"fmt"

	"github.com/google/go-github/v88/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sanguine59/ghlistend/daemon/internal/auth"
	"github.com/sanguine59/ghlistend/daemon/internal/store"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication and daemon state",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	token, err := auth.GetToken()
	if err != nil {
		return err
	}
	ctx := context.Background()
	gh, err := github.NewClient(github.WithAuthToken(token))
	if err != nil {
		return fmt.Errorf("github client: %w", err)
	}
	user, _, err := gh.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	fmt.Printf("authenticated as @%s\n", user.GetLogin())
	if cf := viper.ConfigFileUsed(); cf != "" {
		fmt.Printf("config: %s\n", cf)
	} else {
		fmt.Println("config: (none)")
	}

	st, err := store.Open()
	if err == nil {
		defer st.Close()
		if cp, err := st.LoadCheckpoint(); err == nil {
			fmt.Printf("last_modified: %s\n", cp.LastModified)
			if !cp.LastPollAt.IsZero() {
				fmt.Printf("last_poll_at:  %s\n", cp.LastPollAt.Format("2006-01-02T15:04:05Z07:00"))
			}
		}
	}
	return nil
}
