package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"
)

const stableDiffusionStartTimeout = 30 * time.Second
const stableDiffusionPingInterval = 500 * time.Millisecond

func isStableDiffusionRunning() (bool, error) {
	processes, err := process.Processes()
	if err != nil {
		return false, fmt.Errorf("can't check if stable diffusion is running: %s", err.Error())
	}
	for _, proc := range processes {
		cmdline, err := proc.Cmdline()
		if err != nil {
			continue
		}
		if strings.Contains(cmdline, "python3 launch") {
			return true, nil
		}
	}
	return false, nil
}

func startStableDiffusionIfNeeded(ctx context.Context) error {
	isRunning, err := isStableDiffusionRunning()
	if err != nil {
		return err
	}

	if isRunning {
		fmt.Println("stable diffusion is already running")
	} else {
		fmt.Println("starting stable diffusion... ")
		cmd := exec.Cmd{
			Path: params.StableDiffusionWebUIPath,
			Args: []string{params.StableDiffusionWebUIPath, "--api"},
			Dir:  filepath.Dir(params.StableDiffusionWebUIPath),
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("can't start stable diffusion: %s", err.Error())
		}
	}

	fmt.Println("checking stable diffusion api...")
	startedAt := time.Now()
	var lastPingAt time.Time
	for {
		elapsedSinceLastPing := time.Since(lastPingAt)
		if elapsedSinceLastPing < stableDiffusionPingInterval {
			time.Sleep(stableDiffusionPingInterval - elapsedSinceLastPing)
		}

		_, _, err := sdAPI.GetProgress(ctx)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.ECONNREFUSED) {
			if err.Error() == "Not Found" {
				return fmt.Errorf("can't start stable diffusion, api is not enabled")
			}
			return fmt.Errorf("can't start stable diffusion: %s", err.Error())
		}

		if time.Since(startedAt) > stableDiffusionStartTimeout {
			return fmt.Errorf("can't start stable diffusion: ping timeout")
		}

		lastPingAt = time.Now()
		fmt.Println("  ping...")
	}
	fmt.Println("  ok")
	return nil
}
