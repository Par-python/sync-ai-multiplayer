package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/syncroom/syncroom/internal/cli"
	"github.com/syncroom/syncroom/internal/client"
	"github.com/syncroom/syncroom/internal/domain"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "syncroom:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("missing command")
	}
	switch args[0] {
	case "room":
		return room(args[1:])
	case "attach":
		return attach(args[1:])
	case "watch":
		return watch(args[1:])
	case "intent":
		return intent(args[1:])
	case "decision":
		return decision(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func room(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: syncroom room create --server CONVEX_SITE_URL --name NAME --repo URL --default-branch main")
	}
	fs := flag.NewFlagSet("room create", flag.ContinueOnError)
	serverURL := fs.String("server", "", "Convex site URL")
	name := fs.String("name", "", "room name")
	repo := fs.String("repo", "", "Git remote URL")
	branch := fs.String("default-branch", "main", "protected branch")
	check := fs.String("check", "", "validation command")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	created, err := client.CreateRoom(context.Background(), *serverURL, *name, *repo, *branch, *check)
	if err != nil {
		return err
	}
	fmt.Printf("Room %s created\nJoin code: %s\n", created.ID, created.JoinCode)
	return nil
}
func attach(args []string) error {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	serverURL := fs.String("server", "", "Convex site URL")
	code := fs.String("room", "", "room join code")
	name := fs.String("name", "", "participant name")
	agent := fs.String("agent", "", "agent label")
	branch := fs.String("branch", "", "new branch if current branch is protected")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return cli.Attach(context.Background(), cli.AttachInput{Root: ".", ServerURL: *serverURL, JoinCode: *code, Name: *name, Agent: *agent, Branch: *branch})
}
func watch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.Duration("interval", 2*time.Second, "sync interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return cli.Watch(context.Background(), ".", *interval)
}
func intent(args []string) error {
	fs := flag.NewFlagSet("intent", flag.ContinueOnError)
	task := fs.String("task", "", "task title")
	objective := fs.String("objective", "", "objective")
	areas := fs.String("areas", "", "comma-separated paths")
	status := fs.String("status", "planning", "planning|executing|blocked|done")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return cli.PublishIntent(context.Background(), ".", *task, *objective, *areas, domain.IntentStatus(strings.ToLower(*status)))
}
func decision(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return fmt.Errorf("usage: syncroom decision add --title TITLE --body BODY")
	}
	fs := flag.NewFlagSet("decision add", flag.ContinueOnError)
	title := fs.String("title", "", "decision title")
	body := fs.String("body", "", "decision body")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	return cli.PublishDecision(context.Background(), ".", *title, *body)
}
func printUsage() {
	fmt.Fprintln(os.Stdout, "usage: syncroom <room create|attach|watch|intent|decision add> [flags]")
}
