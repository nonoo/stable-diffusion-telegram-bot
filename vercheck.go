package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v53/github"
)

const versionCheckTimeout = time.Second * 10

func versionCheck(ctx context.Context) (latestVersion, currentVersion string, err error) {
	client := github.NewClient(nil)

	release, _, err := client.Repositories.GetLatestRelease(ctx, "AUTOMATIC1111", "stable-diffusion-webui")
	if err != nil {
		return "", "", fmt.Errorf("getting latest stable diffusion version: %w", err)
	}
	latestVersion = release.GetTagName()

	repo, err := git.PlainOpen(filepath.Dir(params.StableDiffusionWebUIPath))
	if err != nil {
		return "", "", fmt.Errorf("getting current stable diffusion version: %w", err)
	}
	iter, err := repo.Tags()
	if err != nil {
		return "", "", fmt.Errorf("getting current stable diffusion version: %w", err)
	}
	ref, err := iter.Next()
	if err != nil {
		return "", "", fmt.Errorf("getting current stable diffusion version: %w", err)
	}

	return latestVersion, ref.Name().Short(), nil
}

func versionCheckGetStr(ctx context.Context) (res string, updateNeededOrError bool) {
	verCheckCtx, verCheckCtxCancel := context.WithTimeout(ctx, versionCheckTimeout)
	defer verCheckCtxCancel()

	var latestVersion, currentVersion string
	var err error
	if latestVersion, currentVersion, err = versionCheck(verCheckCtx); err != nil {
		return errorStr + ": " + err.Error(), true
	}

	updateNeededOrError = currentVersion != latestVersion
	res = "Stable Diffusion WebUI version: " + currentVersion
	if updateNeededOrError {
		res = "📢 " + res + " 📢 Update needed! Latest version is " + latestVersion + " 📢"
	} else {
		res += " (up to date)"
	}
	return
}
