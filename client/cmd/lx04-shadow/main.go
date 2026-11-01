// SPDX-License-Identifier: Apache-2.0
// Offline-only ASR shadow decisions. Never controls the device.
package main

import (
 "bufio"
 "encoding/csv"
 "flag"
 "fmt"
 "io"
 "os"
 "sort"
 "strings"
 "xiaobot/devicevoice"
)
type session struct { id, question, prediction, reason, observed, detail string }
func predict(q string) (string,string) {
 q=strings.TrimSpace(q)
 lower:=strings.ToLower(strings.ReplaceAll(q," ",""))
 plain:=strings.TrimRight(q,"，。？！!?")
 if plain=="退出" || plain=="退出播放" {return "native-candidate","exit-command"}
 if (strings.HasPrefix(q,"晾衣机") || strings.HasSuffix(q,"晾衣机")) &&
  (strings.Contains(q,"升高") || strings.Contains(q,"下降") || strings.Contains(q,"停止")) {
  return "native-candidate","clothes-rack-control"
 }
 for _,p:=range []string{"问deepseek","让deepseek","用deepseek","deepseek帮","deepseek请","交给deepseek"} {
  if strings.Contains(lower,p) {return "deepseek-explicit","explicit-deepseek"}
 }
 if devicevoice.NativeRequest(q) {return "native-candidate","native-request-rule"}
 for _,p:=range []string{"为什么","什么是","解释一下","介绍一下","帮我写","写一篇","写一首","翻译","总结","分析一下","对比一下","原理"} {
  if strings.Contains(q,p) {return "deepseek-candidate","open-question-rule"}
 }
 return "undetermined","no-sufficient-rule"
}
func update(s *session,e devicevoice.RoutingEvent) {
 if e.Kind=="speak" && s.observed=="" {s.observed="oem-speak";s.detail="SpeechSynthesizer.Speak"}
 if e.Kind!="semantic" {return}
 if e.Namespace=="Template" {
  if devicevoice.IsConfirmedFallback(e) {s.observed="oem-confirmed-fallback";s.detail="Template.Toast failure phrase"} else if s.observed!="oem-confirmed-fallback" {
   s.observed="oem-template";s.detail="Template."+e.Name
  }
 } else if (e.Namespace=="PlaybackController" || e.Namespace=="Dialog") && s.observed=="" {
  s.observed="oem-control";s.detail=e.Namespace+"."+e.Name
 }
}
func analyze(paths []string) ([]*session,error) {
 sessions:=map[string]*session{}
 for _,path:=range paths {
  f,err:=os.Open(path);if err!=nil{return nil,fmt.Errorf("%s: %w",path,err)}
  scanner:=bufio.NewScanner(f);scanner.Buffer(make([]byte,64*1024),2*1024*1024)
  for scanner.Scan() {
   e:=devicevoice.ParseRoutingEvent(scanner.Text())
   if e.DialogID=="" {continue}
   s:=sessions[e.DialogID];if s==nil{s=&session{id:e.DialogID};sessions[e.DialogID]=s}
   if e.Kind=="query" && strings.TrimSpace(e.Text)!="" {
    if s.question=="" {s.question=e.Text;s.prediction,s.reason=predict(e.Text)}
   } else {update(s,e)}
  }
  scanErr:=scanner.Err();closeErr:=f.Close()
  if scanErr!=nil{return nil,fmt.Errorf("%s: %w",path,scanErr)}
  if closeErr!=nil{return nil,closeErr}
 }
 ids:=make([]string,0,len(sessions));for id,s:=range sessions {if s.question!="" {ids=append(ids,id)}}
 sort.Strings(ids)
 out:=make([]*session,0,len(ids));for _,id:=range ids {out=append(out,sessions[id])}
 return out,nil
}
func writeCSV(w io.Writer,sessions []*session) error {
 csvw:=csv.NewWriter(w)
 if err:=csvw.Write([]string{"dialog_id","asr_text","prediction","rule","oem_observed","oem_evidence"});err!=nil{return err}
 for _,s:=range sessions {
  observed:=s.observed;if observed==""{observed="not-observed"}
  if err:=csvw.Write([]string{s.id,s.question,s.prediction,s.reason,observed,s.detail});err!=nil{return err}
 }
 csvw.Flush();return csvw.Error()
}
func main() {
 out:=flag.String("out","","output CSV file path");flag.Parse()
 if *out=="" || flag.NArg()==0 {fmt.Fprintln(os.Stderr,"usage: lx04-shadow-v38.exe -out results.csv mico.log [mico.log.bak.1 ...]");os.Exit(2)}
 ss,err:=analyze(flag.Args());if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
 f,err:=os.Create(*out);if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
 _,err=f.Write([]byte{0xef,0xbb,0xbf});if err==nil{err=writeCSV(f,ss)}
 closeErr:=f.Close();if err==nil{err=closeErr}
 if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
 fmt.Printf("offline shadow complete: %d ASR dialogs -> %s (no device operations)\n",len(ss),*out)
}
