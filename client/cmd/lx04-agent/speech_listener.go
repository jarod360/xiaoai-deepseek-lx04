// SPDX-License-Identifier: Apache-2.0
package main
import ("bufio";"context";"log";"os";"os/exec";"time";"xiaobot/devicevoice")
func retrySpeech(ctx context.Context, read func(context.Context), interrupted func(), delay time.Duration) {
 for ctx.Err()==nil { read(ctx);if ctx.Err()!=nil{return};interrupted();log.Print("local speech listener interrupted; reconnecting");timer:=time.NewTimer(delay);select{case <-ctx.Done():timer.Stop();return;case <-timer.C:} }
}
func (a *agent) listenSpeech(ctx context.Context) {
 cmd:=exec.CommandContext(ctx,"/system/bin/logcat","-v","raw","-T",time.Now().Format("01-02 15:04:05.000"),"MICO.cap:I","MICO.speech:I","*:S")
 stdout,err:=cmd.StdoutPipe();if err!=nil{log.Print("local speech pipe failed");return}
 cmd.Stderr=os.Stderr
 if cmd.Start()!=nil{stdout.Close();log.Print("could not start local speech listener");return}
 defer func(){stdout.Close();cmd.Process.Kill();cmd.Wait()}()
 log.Print("ready: native ASR, automatic routing, on-device DeepSeek client")
 scanner:=bufio.NewScanner(stdout);scanner.Buffer(make([]byte,64*1024),1024*1024)
 for scanner.Scan(){e:=devicevoice.ParseEvent(scanner.Text());switch e.Kind{case "query":a.onQuery(e.Text);case "speak":a.onSpeak(e.Text)}}
 if scanner.Err()!=nil{log.Print("local speech listener read failed")}
}