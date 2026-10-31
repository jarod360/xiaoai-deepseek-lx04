// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"time"
)

const (
	baikeActivity = "com.xiaomi.micolauncher/com.xiaomi.micolauncher.skills.baike.view.BaikeTextActivity"
	wakeupFocus   = "Sys2038:com.xiaomi.micolauncher/com.xiaomi.micolauncher.Launcher"
)

// showNativeAnswer uses the exact Intent extras tested on LX04.
// Do not use a shell: Chinese text, spaces and punctuation remain one extra.
func showNativeAnswer(ctx context.Context, text string) {
	if strings.TrimSpace(text) == "" || ctx.Err() != nil {
		return
	}
	started := time.Now()
	cmd := exec.CommandContext(ctx, "am", "start", "-n", baikeActivity,
		"--es", "KEY_TEXT_CONTENT", text,
		"--es", "KEY_TEXT_TITLE", "DeepSeek")
	// Start bottom-dismiss checks as soon as the Activity launch begins,
	// rather than waiting for "am start" to return.
	if err := cmd.Start(); err != nil {
		log.Printf("native answer screen unavailable: %v", err)
		return
	}
	log.Printf("native UI activity launched in %s", time.Since(started).Round(time.Millisecond))
	dismissCtx, stopDismiss := context.WithCancel(ctx)
	go func() {
		if err := cmd.Wait(); err != nil {
			stopDismiss()
			log.Printf("native answer screen launch failed: %v", err)
		}
	}()
	go dismissNativeBottom(dismissCtx, started)
}

// BACK uses the independently verified uinput probe on LX04.
// The helper rechecks focus immediately before injecting the key.
// BACK is allowed only while the original LX04 wakeup overlay owns focus.
// If the full-screen answer activity owns focus, never send BACK.
func dismissNativeBottom(ctx context.Context, started time.Time) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("native bottom check cancelled after %s", time.Since(started).Round(time.Millisecond))
			return
		case <-timer.C:
			log.Printf("native bottom focus not found after %s", time.Since(started).Round(time.Millisecond))
			return
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
			out, err := exec.CommandContext(checkCtx, "dumpsys", "window", "windows").Output()
			cancel()
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(out), "\n") {
				if !strings.Contains(line, "mCurrentFocus=") || !strings.Contains(line, wakeupFocus) {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				log.Printf("native bottom focus matched after %s", time.Since(started).Round(time.Millisecond))
				backStarted := time.Now()
				if err := exec.CommandContext(ctx, "/data/local/tmp/xiaoseek-test/back-test-bin").Run(); err != nil {
					log.Printf("native bottom dismiss failed after %s: %v", time.Since(started).Round(time.Millisecond), err)
				} else {
					log.Printf("native bottom BACK completed after %s (command %s)", time.Since(started).Round(time.Millisecond), time.Since(backStarted).Round(time.Millisecond))
				}
				return
			}
		}
	}
}
