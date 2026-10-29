package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/corn12138/yan-source/internal/releasechannel"
	"github.com/corn12138/yan-source/internal/updatepack"
)

func run() error {
	dir := flag.String("dir", "", "verified numbered release directory")
	tag := flag.String("tag", "", "release tag")
	target := flag.String("target", "darwin-arm64", "update platform target")
	flag.Parse()
	kind, err := updatepack.Channel(*tag)
	if err != nil {
		return err
	}
	if len(flag.Args()) == 0 || *target == "" || strings.ContainsAny(*target, "/\\") {
		return fmt.Errorf("expected update channels and a platform target")
	}
	for _, channel := range flag.Args() {
		if channel != "stable" && channel != "preview" || kind == "preview" && channel == "stable" {
			return fmt.Errorf("invalid update channel for release")
		}
	}
	name := "update-" + *target + ".json"
	data, err := os.ReadFile(filepath.Join(*dir, name))
	if err != nil {
		return err
	}
	var m updatepack.Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return err
	}
	archive := "yan-" + *tag + "-" + *target + ".update.tar.gz"
	if m.Version != strings.TrimPrefix(*tag, "v") || m.URL != "https://github.com/"+updatepack.Repository+"/releases/download/"+*tag+"/"+archive {
		return fmt.Errorf("manifest differs from numbered release")
	}
	var cfg struct {
		Updates struct{ PublicKey string } `json:"updates"`
	}
	config, err := os.ReadFile("mygo.json")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(config, &cfg); err != nil {
		return err
	}
	if err = updatepack.Verify(filepath.Join(*dir, archive), m, cfg.Updates.PublicKey); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	for _, channel := range flag.Args() {
		result, err := releasechannel.Promote(ctx, releasechannel.GitHub{}, channel, *tag, name, data)
		if err != nil {
			return fmt.Errorf("%s: %w", channel, err)
		}
		fmt.Printf("Channel %s: %s\n", channel, result)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "promoteupdates:", err)
		os.Exit(1)
	}
}
