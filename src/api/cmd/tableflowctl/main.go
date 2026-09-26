// Command tableflowctl performs operator tasks against the API database.
//
//	tableflowctl bootstrap-branch -name "Main" -email manager@example.com -display-name "Manager"
//
// bootstrap-branch creates the first branch and an invited manager, then
// prints a one-time activation token. It refuses once any branch exists.
// DATABASE_URL uses the restricted application role.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/database"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/password"
	"github.com/epic-rinn/tableflow/src/api/internal/seating"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tableflowctl:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "bootstrap-branch" {
		return errors.New("usage: tableflowctl bootstrap-branch -name NAME -email EMAIL -display-name NAME")
	}
	fs := flag.NewFlagSet("bootstrap-branch", flag.ContinueOnError)
	name := fs.String("name", "", "branch name")
	email := fs.String("email", "", "first manager's email")
	display := fs.String("display-name", "", "first manager's display name")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, dbURL, database.Options{
		MaxConns: 2, StatementTimeout: 10 * time.Second, LockTimeout: 5 * time.Second, IdleInTxTimeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	svc := identity.NewService(pool, password.New(1), time.Hour, 12*time.Hour)
	branchID, staff, act, err := svc.Bootstrap(ctx, *name, *email, *display)
	var ve *identity.ValidationError
	if errors.As(err, &ve) {
		return fmt.Errorf("invalid input: %v", ve.Fields)
	}
	if err != nil {
		return err
	}
	if err := seating.InsertDefaultGroups(ctx, pool, branchID); err != nil {
		return fmt.Errorf("default seating groups: %w", err)
	}
	// Output is for the operator's terminal only; the token is single-use
	// and expires. Deliver the link to the manager over a private channel.
	fmt.Printf("branch_id=%s\nmanager_id=%s\nactivation_token=%s\nactivation_expires_at=%s\n",
		branchID, staff.ID, act.Token, act.ExpiresAt.UTC().Format(time.RFC3339))
	fmt.Println("Open <admin origin>/activate#<activation_token> to set the manager's password.")
	return nil
}
