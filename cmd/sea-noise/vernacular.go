package main

import (
	"context"
	"flag"
	"fmt"

	"sea-noise/internal/config"
	"sea-noise/internal/vernacular"
)

func runVernacular(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sea-noise vernacular link --key KEY --project CHECKOUT | unlink KEY | list")
	}
	action := args[0]
	fs := flag.NewFlagSet("vernacular "+action, flag.ContinueOnError)
	var key, checkout string
	if action == "link" {
		fs.StringVar(&key, "key", "", "single-segment project name in the vault wing")
		fs.StringVar(&checkout, "project", "", "project checkout with an initialized Vernacular workspace")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch action {
	case "link":
		if key == "" || checkout == "" || fs.NArg() != 0 {
			return fmt.Errorf("usage: sea-noise vernacular link --key KEY --project CHECKOUT")
		}
	case "unlink":
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: sea-noise vernacular unlink KEY")
		}
		key = fs.Arg(0)
	case "list":
		if fs.NArg() != 0 {
			return fmt.Errorf("usage: sea-noise vernacular list")
		}
	default:
		return fmt.Errorf("unknown vernacular action %q; use link, unlink or list", action)
	}

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}
	switch action {
	case "link":
		project, err := vernacular.Attach(context.Background(), vaultRoot, key, checkout)
		if err != nil {
			return err
		}
		fmt.Printf("Linked %s\n  project: %s\n  overview: %s\n", project.Key, project.Canon, project.Overview)
	case "unlink":
		if err := vernacular.Detach(vaultRoot, key); err != nil {
			return err
		}
		fmt.Printf("Unlinked %s; original project notes preserved\n", key)
	case "list":
		projects, err := vernacular.List(vaultRoot)
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Println("(no linked projects)")
		}
		for _, project := range projects {
			fmt.Printf("%s\t%s\t%s\n", project.Key, project.Canon, project.Overview)
		}
	}
	return nil
}
