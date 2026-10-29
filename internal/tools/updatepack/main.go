package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/internal/updatepack"
)

func run() error {
	bundle := flag.String("bundle", "", "application directory")
	out := flag.String("out", "", "release output directory")
	tag := flag.String("tag", "", "release tag")
	target := flag.String("target", "darwin-arm64", "MyGo update target")
	notes := flag.String("notes", "", "release notes file")
	check := flag.Bool("check", false, "verify existing manifest and archive")
	previous := flag.String("promote-from", "", "previous channel manifest")
	flag.Parse()
	if *previous != "" {
		b, err := os.ReadFile(*previous)
		if err != nil {
			return err
		}
		var m updatepack.Manifest
		if err = json.Unmarshal(b, &m); err != nil {
			return err
		}
		ok, err := updatepack.CanPromote(m.Version, *tag)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("yes")
		} else {
			fmt.Println("no")
		}
		return nil
	}
	var cfg struct {
		Updates struct {
			PublicKey string `json:"publicKey"`
			URL       string `json:"url"`
		} `json:"updates"`
	}
	b, err := os.ReadFile("mygo.json")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	channel, err := updatepack.Channel(*tag)
	if err != nil {
		return err
	}
	if cfg.Updates.URL != updatepack.Feed(channel) {
		return fmt.Errorf("build feed must be %s", updatepack.Feed(channel))
	}
	if *check {
		b, err = os.ReadFile(filepath.Join(*out, "update-"+*target+".json"))
		if err != nil {
			return err
		}
		var m updatepack.Manifest
		if err = json.Unmarshal(b, &m); err != nil {
			return err
		}
		if m.Version != strings.TrimPrefix(*tag, "v") || m.URL != "https://github.com/"+updatepack.Repository+"/releases/download/"+*tag+"/yan-"+*tag+"-"+*target+".update.tar.gz" {
			return fmt.Errorf("manifest version or archive URL differs from release")
		}
		return updatepack.Verify(filepath.Join(*out, "yan-"+*tag+"-"+*target+".update.tar.gz"), m, cfg.Updates.PublicKey)
	}
	secret := os.Getenv("MYGO_UPDATER_PRIVATE_KEY")
	if secret == "" {
		path := os.Getenv("YAN_UPDATE_KEY_FILE")
		if path == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path = filepath.Join(home, ".config", "yan", "update-keys", "mygo-update.key")
		}
		b, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read update signing key: %w", err)
		}
		secret = string(b)
	}
	key, err := updatepack.Key(secret, cfg.Updates.PublicKey)
	if err != nil {
		return err
	}
	b, err = os.ReadFile(*notes)
	if err != nil {
		return err
	}
	_, err = updatepack.Pack(*bundle, *out, *tag, *target, string(b), key)
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "updatepack:", err)
		os.Exit(1)
	}
}
