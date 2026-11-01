package main
import (
 "bytes"
 "strings"
 "testing"
 "xiaobot/devicevoice"
)
func TestPredictionsConservative(t *testing.T) {
 for _,c:=range []struct{q,want string}{
  {"现在几点了","native-candidate"},
  {"帮我打开卧室灯","native-candidate"},
  {"退出","native-candidate"},
  {"退出播放","native-candidate"},
  {"晾衣机停止","native-candidate"},
  {"升高晾衣机","native-candidate"},
  {"今天下雨吗","native-candidate"},
  {"为什么天空是蓝色的","deepseek-candidate"},
  {"让DeepSeek帮我写一首诗","deepseek-explicit"},
  {"你知道延迟器吗","undetermined"},
 } {
  got,_:=predict(c.q);if got!=c.want {t.Errorf("%q got %q, want %q",c.q,got,c.want)}
 }
}
func TestEvidenceIsNotTruthLabel(t *testing.T) {
 s:=&session{}
 update(s,devicevoice.RoutingEvent{Kind:"semantic",Namespace:"Template",Name:"General",Text:"如果汪洋是蓝色的"})
 if s.observed!="oem-template" {t.Fatal("Template.General is an observation, not a correctness verdict")}
 update(s,devicevoice.RoutingEvent{Kind:"semantic",Namespace:"Template",Name:"Toast",Text:"被难住了诶"})
 if s.observed!="oem-confirmed-fallback" {t.Fatal("failure evidence should win")}
 var b bytes.Buffer;if err:=writeCSV(&b,[]*session{{id:"abc",question:"你好",prediction:"undetermined",observed:s.observed}});err!=nil{t.Fatal(err)}
 if !strings.Contains(b.String(),"oem-confirmed-fallback") {t.Fatal("missing evidence")}
}
