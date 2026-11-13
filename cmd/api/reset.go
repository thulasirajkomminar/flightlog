package main

import (
	"context"
	"fmt"
	"os"

	"github.com/thulasirajkomminar/flightlog/internal/config"
	"github.com/thulasirajkomminar/flightlog/internal/database"
	"github.com/thulasirajkomminar/flightlog/internal/user"
)

const (
	resetPasswordCommand = "reset-password"
	// resetArgs is the argument count of `<binary> reset-password <email>`,
	// and emailArg is where the email sits in it.
	resetArgs = 3
	emailArg  = 2
)

// isResetPassword reports whether the binary was started as
// `<binary> reset-password ...` rather than as the server.
func isResetPassword(args []string) bool {
	return len(args) > 1 && args[1] == resetPasswordCommand
}

// runResetPassword sets a new random password for the account with the given
// email and prints it. It is the recovery path for a forgotten password:
// there is no email flow, but whoever can run the binary owns the instance.
func runResetPassword(cfg *config.Config, args []string) error {
	if len(args) != resetArgs {
		return fmt.Errorf("%w: %s %s <email>", errUsage, args[0], resetPasswordCommand)
	}

	email := args[emailArg]

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	password, err := user.NewService(user.NewStore(db)).ResetPassword(context.Background(), email)
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "New password for %s: %s\nSign in and change it under Settings. Existing sessions have been signed out.\n", email, password)

	return nil
}
