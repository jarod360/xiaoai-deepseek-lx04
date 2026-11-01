// SPDX-License-Identifier: Apache-2.0
package main
import ("bufio";"context";"log";"os";"os/exec";"time";"xiaobot/devicevoice")
func retrySpeech(ctx context.Context, read func(context.Context), interrupted func(), delay time.Duration) {
 for ctx.Err()==nil { read(ctx);if ctx.Err()!=nil{return};interrupted();log.Print("local speech listener interrupted; reconnecting");timer:=time.NewTimer(delay);select{case <-ctx.Done():timer.Stop();return;case <-timer.C:} }
}
func (a *agent) listenSpeech(ctx context.Context) {
 cmd:=exec.CommandContext(ctx,"/system/bin/logcat","-v","time","-T",time.Now().Format("01-02 15:04:05.000"),"MICO.cap:I","MICO.speech:D","*:S")
 stdout,err:=cmd.StdoutPipe();if err!=nil{log.Print("local speech pipe failed");return}
 cmd.Stderr=os.Stderr
 if err=cmd.Start();err!=nil{stdout.Close();log.Printf("could not start local speech listener: %v",err);return}
 started:=time.Now()
 pid:=cmd.Process.Pid
 log.Printf("local speech listener started pid=%d",pid)
 log.Print("ready: native ASR, automatic routing, on-device DeepSeek client")
 scanner:=bufio.NewScanner(stdout);scanner.Buffer(make([]byte,64*1024),1024*1024)
 for scanner.Scan(){
  e:=devicevoice.ParseRoutingEvent(scanner.Text())
  switch e.Kind {
  case "query": a.onRoutingQuery(e)
  case "speak": a.onSpeak(e.Text); a.onRoutingEvent(e)
  case "semantic","play-start","play-finish","tts-end": a.onRoutingEvent(e)
  }
 }
 readErr:=scanner.Err()
 // Capture the child's real exit status if it exits promptly after EOF.
 // Otherwise perform the same child cleanup as before, without blocking reconnection.
 waited:=make(chan error,1)
 go func(){waited<-cmd.Wait()}()
 var exitErr error
 forced:=false
 select{
 case exitErr=<-waited:
 case <-time.After(200*time.Millisecond):
  forced=true
  _=cmd.Process.Kill()
  exitErr=<-waited
 }
 _=stdout.Close()
 log.Printf("local speech listener ended pid=%d duration=%s read_error=%v exit=%v forced_cleanup=%t",pid,time.Since(started).Round(time.Millisecond),readErr,exitErr,forced)
}