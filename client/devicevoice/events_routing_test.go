package devicevoice
import "testing"
func TestRoutingFinalASRAndDuplicate(t *testing.T) {
 line:="11-01 12:44:49.366 I/MICO.cap(27572): instruction={\"header\":{\"namespace\":\"SpeechRecognizer\",\"name\":\"RecognizeResult\",\"dialog_id\":\"abc31\"},\"payload\":{\"is_final\":true,\"results\":[{\"text\":\"我要晾衣服\"}]}}"
 e:=ParseRoutingEvent(line)
 if e.Kind!="query" || e.DialogID!="abc31" || e.Text!="我要晾衣服" {t.Fatalf("%+v",e)}
 if x:=ParseRoutingEvent("I/MICO.speech: MSG_ASR_FINAL_RESULT:我要晾衣服");x.Kind!="" {t.Fatalf("duplicate: %+v",x)}
}
func TestRoutingFallbackSpecificity(t *testing.T) {
 for _,tc:=range []struct{task,name string;want bool}{
  {"2424b468fff5dfa723ab2b4862f48cfd","Toast",true},
  {"43234f6eb43a7e551ebc1f57a5b5bc75","Toast",true},
  {"e609437d68f98af5e45b41ed6ffd20f7","Toast",false},
  {"85fc377563f3feda157654b9a1042f14","Toast",false},
  {"2424b468fff5dfa723ab2b4862f48cfd","General",false},
 } {
  e:=RoutingEvent{Kind:"semantic",Namespace:"Template",Name:tc.name,Task:tc.task}
  if got:=IsConfirmedFallback(e);got!=tc.want {t.Errorf("task=%s got=%t",tc.task,got)}
 }
}
func TestRoutingPlaybackCallbacks(t *testing.T) {
 e:=ParseRoutingEvent("D/MICO.speech(27572): onPlayStart=abc31")
 if e.Kind!="play-start" || e.DialogID!="abc31" {t.Fatal(e)}
 if x:=ParseRoutingEvent("D/MICO.speech(27572): onPlayFinish");x.Kind!="play-finish" {t.Fatal(x)}
}
