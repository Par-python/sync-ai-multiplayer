package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/syncroom/syncroom/internal/cli"
	"github.com/syncroom/syncroom/internal/domain"
	"github.com/syncroom/syncroom/internal/server"
	"github.com/syncroom/syncroom/internal/store"
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
	case "serve":
		return serve(args[1:])
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

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	data := fs.String("data", "syncroom.db", "SQLite database path")
	listen := fs.String("listen", ":8080", "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := store.Open(*data)
	if err != nil {
		return err
	}
	defer db.Close()
	fmt.Printf("Syncroom coordinator listening on %s\n", *listen)
	return http.ListenAndServe(*listen, server.New(db))
}
func room(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: syncroom room create --data DB --name NAME --repo URL --default-branch main")
	}
	fs := flag.NewFlagSet("room create", flag.ContinueOnError)
	data := fs.String("data", "syncroom.db", "SQLite database path")
	name := fs.String("name", "", "room name")
	repo := fs.String("repo", "", "Git remote URL")
	branch := fs.String("default-branch", "main", "protected branch")
	check := fs.String("check", "", "validation command")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	db, err := store.Open(*data)
	if err != nil {
		return err
	}
	defer db.Close()
	created, err := db.CreateRoom(context.Background(), store.CreateRoomInput{Name: *name, Repo: *repo, DefaultBranch: *branch, CheckCommand: *check})
	if err != nil {
		return err
	}
	fmt.Printf("Room %s created\nJoin code: %s\n", created.ID, created.JoinCode)
	return nil
}
func attach(args []string) error {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	serverURL := fs.String("server", "", "coordinator URL")
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
	fmt.Fprintln(os.Stdout, "usage: syncroom <serve|room create|attach|watch|intent|decision add> [flags]")
}
