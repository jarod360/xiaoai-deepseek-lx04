// SPDX-License-Identifier: Apache-2.0
package main

import (
 "context"
 "log"
 "os/exec"
 "strings"
 "sync"
 "time"
 "xiaobot/devicevoice"
)

// Observed-data window, not a universal bound on future Xiaomi skills.
const routeWindow = 1800 * time.Millisecond
type routeSession struct {
 id,query string
 timer *time.Timer
 fallback,played,sawSpeak,decided,stopIssued bool
}
type routeState struct {
 mu sync.Mutex
 current *routeSession
 playID string
 ambiguous bool
}

// Only CAP final ASR with a full dialog ID starts a new routing session.
func (a *agent) onRoutingQuery(e devicevoice.RoutingEvent) {
 if a.doorbell!=nil && a.doorbell.check(context.Background()) {
  a.routing.abortPending()
  a.preemptForDoorbell()
  return
 }
 q:=strings.TrimSpace(e.Text)
 if q=="" || e.DialogID=="" {return}
 r:=a.routing
 r.mu.Lock()
 defer r.mu.Unlock()
 if r.current!=nil && r.current.id==e.DialogID {return}
 if r.current!=nil && r.current.timer!=nil {r.current.timer.Stop()}
 s:=&routeSession{id:e.DialogID,query:q}
 r.current=s
 a.mu.Lock()
 a.generation++
 if a.cancel!=nil {a.cancel();a.cancel=nil}
 a.pending=""
 a.restore()
 a.mu.Unlock()
 // These commands must never enter AI; the native assistant still sees the ASR.
 if isExitRequest(q) {
  s.decided=true
  log.Printf("route=exit; dialog=%s",s.id)
  return
 }
 if devicevoice.NativeRequest(q) {
  s.decided=true
  log.Printf("route=native; reason=fast-control; dialog=%s",s.id)
  return
 }
 if q=="返回桌面" || q=="回到桌面" {
  r.acceptNative(s,"local-navigation")
  return
 }
 s.timer=time.AfterFunc(routeWindow,func(){
  r.mu.Lock()
  defer r.mu.Unlock()
  r.resolveTimeout(a,s)
 })
}
func (r *routeState) resolveTimeout(a *agent,s *routeSession) {
 if r.current!=s || s.decided || s.fallback {return}
 // A TTS from any dialog blocks an unconditional takeover.
 if s.sawSpeak || r.playID!="" || r.ambiguous {
  r.acceptNative(s,"native-or-ongoing-audio")
  return
 }
 s.decided=true
 log.Printf("routing_decision=deepseek; reason=no-native-evidence; dialog=%s",s.id)
 a.onQuery(s.query)
}
func (r *routeState) abortPending() {
 r.mu.Lock()
 defer r.mu.Unlock()
 if r.current!=nil && r.current.timer!=nil {r.current.timer.Stop()}
 r.current=nil
 r.playID=""
 r.ambiguous=false
}
func isExitRequest(q string) bool {
 q=strings.TrimRight(strings.TrimSpace(q),"，。？！!?, ")
 return q=="退出" || q=="退出播放"
}
func (r *routeState) acceptNative(s *routeSession,why string) {
 if s.decided {return}
 s.decided=true
 if s.timer!=nil {s.timer.Stop()}
 log.Printf("route=native; reason=%s; dialog=%s",why,s.id)
}
// Only the confirmed failure text of the *currently playing* dialog may
// request an OEM TTS stop. Ordinary Toasts, controls and overlapping audio are excluded.
func (r *routeState) canStopFallback(s *routeSession) bool {
 return s!=nil && r.current==s && !s.decided && s.fallback && !s.stopIssued &&
  r.playID==s.id && !r.ambiguous
}

// Call only with routeState.mu held. Never block the logcat reader on am start.
func (a *agent) scheduleFallbackStop(s *routeSession) {
 if !a.experimentalStopFallback || !a.routing.canStopFallback(s) {return}
 s.stopIssued=true
 log.Printf("experimental_fallback_stop=scheduled; dialog=%s",s.id)
 go a.stopFallbackTTS(s)
}

// An ADB-verified OEM command. No shell, no modifications to the stock APK.
// Still non-dialog-scoped inside Xiaomi: the preflight guard is best-effort.
func (a *agent) stopFallbackTTS(s *routeSession) {
 r:=a.routing
 r.mu.Lock()
 if r.current!=s || s.decided || !s.fallback || r.playID!=s.id || r.ambiguous {
  r.mu.Unlock()
  log.Printf("experimental_fallback_stop=skipped_stale; dialog=%s",s.id)
  return
 }
 ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second)
 cmd:=exec.CommandContext(ctx,"am","start",
  "-a","com.xiaomi.micolauncher.UBUS",
  "-n","com.xiaomi.micolauncher/.common.schema.SchemaActivity",
  "--es","path","mibrain",
  "--es","method","ai_service",
  "--es","payload",`{"for_test":true,"nlp_text":""}`)
 started:=time.Now()
 err:=cmd.Start()
 r.mu.Unlock()
 if err==nil {err=cmd.Wait()}
 cancel()
 if err!=nil {
  log.Printf("experimental_fallback_stop=command_error; dialog=%s elapsed=%s err=%v",
   s.id,time.Since(started).Round(time.Millisecond),err)
 } else {
  // am start can return before UBus handles the Intent: not TTS-stop latency.
  log.Printf("experimental_fallback_stop=am_accepted; dialog=%s elapsed=%s",
   s.id,time.Since(started).Round(time.Millisecond))
 }
}

func (r *routeState) acceptFallback(a *agent,s *routeSession) {
 if s.decided || !s.fallback || !s.played || r.playID!="" || r.ambiguous {return}
 s.decided=true
 if s.timer!=nil {s.timer.Stop()}
 log.Printf("routing_decision=deepseek; reason=confirmed-fallback-after-tts; dialog=%s",s.id)
 a.onQuery(s.query)
}
func (a *agent) onRoutingEvent(e devicevoice.RoutingEvent) {
 r:=a.routing
 r.mu.Lock()
 defer r.mu.Unlock()
 s:=r.current
 switch e.Kind {
 case "play-start":
  if r.playID!="" {r.ambiguous=true}
  r.playID=e.DialogID
  a.scheduleFallbackStop(s)
  return
 case "play-finish":
  id:=r.playID
  if id=="" {return}
  if r.ambiguous {
   r.playID=""
   r.ambiguous=false
   return
  }
  r.playID=""
  if s!=nil && id==s.id {
   s.played=true
   r.acceptFallback(a,s)
  }
  return
 case "semantic","speak":
  if s==nil || s.decided || e.DialogID!=s.id {return}
  if e.Kind=="speak" {
   // Speak can arrive before a fallback Toast. It is not a success verdict.
   s.sawSpeak=true
   return
  }
  if e.Namespace=="Template" {
   if devicevoice.IsConfirmedFallback(e) {
    s.fallback=true
    a.scheduleFallbackStop(s)
    r.acceptFallback(a,s)
   } else {
    r.acceptNative(s,"native-template-"+e.Name)
   }
  } else if e.Namespace=="PlaybackController" || (e.Namespace=="Dialog" && e.Name=="ExitContinuousDialog") {
   r.acceptNative(s,"native-control-"+e.Namespace+"."+e.Name)
  }
 }
}
