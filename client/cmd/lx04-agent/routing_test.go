package main
import (
 "testing"
 "xiaobot/devicevoice"
)
func TestExitRequestFastRoute(t *testing.T) {
 for _,q:=range []string{"退出","退出。","退出！","退出播放","退出播放。","退出播放！"} {
  if !isExitRequest(q) {t.Fatalf("missed exit %q",q)}
 }
 for _,q:=range []string{"为什么要退出播放","退出播放后会怎样","退出设备管理"} {
  if isExitRequest(q) {t.Fatalf("false exit %q",q)}
 }
}
func TestSessionNativeAndReset(t *testing.T) {
 r:=&routeState{}
 s:=&routeSession{id:"scene"}
 r.current=s
 r.acceptNative(s,"test")
 if !s.decided {t.Fatal("native not finalized")}
 r.abortPending()
 if r.current!=nil || r.playID!="" {t.Fatal("session not cleared")}
}
func TestRoutingFallbackEvidence(t *testing.T) {
 for _,tc:=range []struct{task,text string;want bool}{
  {"2424b468fff5dfa723ab2b4862f48cfd","被难住了诶，看来我还要再学习一下",true},
  {"43234f6eb43a7e551ebc1f57a5b5bc75","这个问题我还在学习中， 先聊点别的吧",true},
  {"c21f969b5f03d33d43e04f8f136e7682","对不起， 我还在学习中!",true},
  {"85fc377563f3feda157654b9a1042f14","正在升高晾衣机。",false},
  {"2424b468fff5dfa723ab2b4862f48cfd","好啊。",false},
 } {
  e:=devicevoice.RoutingEvent{Kind:"semantic",Namespace:"Template",Name:"Toast",Task:tc.task,Text:tc.text}
  if got:=devicevoice.IsConfirmedFallback(e);got!=tc.want {t.Errorf("task=%s text=%q got=%t",tc.task,tc.text,got)}
 }
}
