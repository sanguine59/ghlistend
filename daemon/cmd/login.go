package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/go-github/v88/github"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sanguine59/ghlistend/daemon/internal/auth"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store a GitHub Personal Access Token in the OS keyring",
	RunE:  runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	token, err := readToken()
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("empty token")
	}

	ctx := context.Background()
	gh, err := github.NewClient(github.WithAuthToken(token))
	if err != nil {
		return fmt.Errorf("github client: %w", err)
	}
	user, _, err := gh.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	if err := auth.SetToken(token); err != nil {
		return fmt.Errorf("store token: %w", err)
	}
	fmt.Printf("authenticated as @%s\n", user.GetLogin())
	return nil
}

func readToken() (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "GitHub PAT: ")
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
