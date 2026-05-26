package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sanguine59/ghlistend/daemon/internal/auth"
	"github.com/sanguine59/ghlistend/daemon/internal/notifier"
	"github.com/sanguine59/ghlistend/daemon/internal/poller"
	"github.com/sanguine59/ghlistend/daemon/internal/store"
)

var notifyExisting bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Run the notification daemon in the foreground",
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVar(&notifyExisting, "notify-existing", false, "fire toasts for notifications already unread on first run")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	token, err := auth.GetToken()
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	st, err := store.Open()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	nt, err := notifier.New()
	if err != nil {
		return fmt.Errorf("init notifier: %w", err)
	}
	defer nt.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	p, err := poller.New(ctx, poller.Options{
		Token:          token,
		NotifyExisting: notifyExisting,
		Logger:         log,
	}, st, nt)
	if err != nil {
		return fmt.Errorf("init poller: %w", err)
	}

	log.Info("ghlistend starting")
	if err := p.Run(ctx); err != nil && err != context.Canceled {
		// A bad/expired token is terminal: exit cleanly so systemd's
		// Restart=on-failure does not loop us into a re-auth toast storm.
		// The poller has already notified the user to run `ghlistend login`.
		if errors.Is(err, poller.ErrUnauthorized) {
			log.Info("ghlistend stopped; awaiting re-authentication")
			return nil
		}
		return err
	}
	log.Info("ghlistend stopped")
	return nil
}
