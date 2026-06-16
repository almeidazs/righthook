package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/almeidazs/righthook/internal/version"
	"github.com/creativeprojects/go-selfupdate"
)

const SLUG = "almeidazs/righthook"

func Update() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

	defer cancel()
	
	latest, found, err := selfupdate.DetectLatest(ctx, selfupdate.ParseSlug(SLUG))

	if err != nil {
		return fmt.Errorf("couldn’t check for updates: %w", err)
	}

	if !found || latest.LessOrEqual(version.Version) {
		fmt.Printf("You’re already on the latest version (%s)\n", version.Version)
		
		return nil
	}

	fmt.Printf("Update available: %s\n", latest.Version())
	fmt.Println("Downloading and installing...")

	exe, _ := os.Executable()

	if err := selfupdate.UpdateTo(ctx, latest.AssetURL, latest.AssetName, exe); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("Righthook is now in the version %s", latest.Version())

	return nil
}
