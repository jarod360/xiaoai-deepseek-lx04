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
 for _,task:=range []string{"2424b468fff5dfa723ab2b4862f48cfd","43234f6eb43a7e551ebc1f57a5b5bc75"} {
  e:=devicevoice.RoutingEvent{Kind:"semantic",Namespace:"Template",Name:"Toast",Task:task}
  if !devicevoice.IsConfirmedFallback(e) {t.Fatal(task)}
 }
}
