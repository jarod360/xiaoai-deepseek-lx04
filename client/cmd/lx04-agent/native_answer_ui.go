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

// Show the original LX04 answer screen, without letting a late BACK hit it.
// The tested uinput BACK is completed (or skipped) before this Activity starts.
func showNativeAnswer(ctx context.Context, text string) {
 if strings.TrimSpace(text)=="" || ctx.Err()!=nil {return}
 dismissNativeBottom(ctx,time.Now())
 if ctx.Err()!=nil {return}

 started:=time.Now()
 cmd:=exec.CommandContext(ctx,"am","start","-n",baikeActivity,
  "--es","KEY_TEXT_CONTENT",text,
  "--es","KEY_TEXT_TITLE","DeepSeek")
 if err:=cmd.Start();err!=nil {
  log.Printf("native answer screen unavailable: %v",err)
  return
 }
 log.Printf("native UI activity launched in %s",time.Since(started).Round(time.Millisecond))
 go func(){
  if err:=cmd.Wait();err!=nil {log.Printf("native answer screen launch failed: %v",err)}
 }()
}

// Only trust the CURRENT focus line, not historical window entries.
func hasWakeupFocus(out string) bool {
 for _,line:=range strings.Split(out,"\n") {
  if strings.Contains(line,"mCurrentFocus=") && strings.Contains(line,wakeupFocus) {return true}
 }
 return false
}

// This is a one-shot, bounded pre-launch check. When focus is not clearly
// the launcher wakeup overlay, skip BACK rather than risk exiting another UI.
func dismissNativeBottom(ctx context.Context, started time.Time) {
 checkCtx,cancel:=context.WithTimeout(ctx,800*time.Millisecond)
 out,err:=exec.CommandContext(checkCtx,"dumpsys","window","windows").Output()
 cancel()
 if err!=nil {
  log.Printf("native bottom focus unavailable; BACK skipped: %v",err)
  return
 }
 if !hasWakeupFocus(string(out)) {
  log.Print("native bottom focus not matched; BACK skipped")
  return
 }
 if ctx.Err()!=nil {return}
 log.Printf("native bottom focus matched after %s",time.Since(started).Round(time.Millisecond))
 backStarted:=time.Now()
 backCtx,stopBack:=context.WithTimeout(ctx,3*time.Second)
 err=exec.CommandContext(backCtx,"/data/local/tmp/xiaoseek-test/back-test-bin").Run()
 stopBack()
 if err!=nil {
  log.Printf("native bottom dismiss failed after %s: %v",time.Since(started).Round(time.Millisecond),err)
 } else {
  log.Printf("native bottom BACK completed after %s (command %s)",time.Since(started).Round(time.Millisecond),time.Since(backStarted).Round(time.Millisecond))
 }
}
