package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v53/github"
)

const versionCheckTimeout = time.Second * 10

func versionCheckGetLatestTagFromRepository(repository *git.Repository) (string, error) {
	tagRefs, err := repository.Tags()
	if err != nil {
		return "", err
	}

	var latestTagCommit *object.Commit
	var latestTagName string
	err = tagRefs.ForEach(func(tagRef *plumbing.Reference) error {
		revision := plumbing.Revision(tagRef.Name().String())
		tagCommitHash, err := repository.ResolveRevision(revision)
		if err != nil {
			return err
		}

		commit, err := repository.CommitObject(*tagCommitHash)
		if err != nil {
			return err
		}

		if latestTagCommit == nil {
			latestTagCommit = commit
			latestTagName = tagRef.Name().Short()
		}

		if commit.Committer.When.After(latestTagCommit.Committer.When) {
			latestTagCommit = commit
			latestTagName = tagRef.Name().Short()
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return latestTagName, nil
}

func versionCheck(ctx context.Context) (latestVersion, currentVersion string, err error) {
	client := github.NewClient(nil)

	release, _, err := client.Repositories.GetLatestRelease(ctx, "AUTOMATIC1111", "stable-diffusion-webui")
	if err != nil {
		return "", "", fmt.Errorf("getting latest stable diffusion version: %w", err)
	}
	latestVersion = release.GetTagName()

	repo, err := git.PlainOpen(params.StableDiffusionPath)
	if err != nil {
		return "", "", fmt.Errorf("getting current stable diffusion version: %w", err)
	}
	currentVersion, err = versionCheckGetLatestTagFromRepository(repo)
	if err != nil {
		return "", "", fmt.Errorf("getting current stable diffusion version: %w", err)
	}

	return latestVersion, currentVersion, nil
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
