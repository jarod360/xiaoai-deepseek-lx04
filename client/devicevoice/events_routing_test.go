package devicevoice
import "testing"
func TestRoutingFinalASRAndDuplicate(t *testing.T) {
 line:="11-01 12:44:49.366 I/MICO.cap(27572): instruction={\"header\":{\"namespace\":\"SpeechRecognizer\",\"name\":\"RecognizeResult\",\"dialog_id\":\"abc31\"},\"payload\":{\"is_final\":true,\"results\":[{\"text\":\"我要晾衣服\"}]}}"
 e:=ParseRoutingEvent(line)
 if e.Kind!="query" || e.DialogID!="abc31" || e.Text!="我要晾衣服" {t.Fatalf("%+v",e)}
 if x:=ParseRoutingEvent("I/MICO.speech: MSG_ASR_FINAL_RESULT:我要晾衣服");x.Kind!="" {t.Fatalf("duplicate: %+v",x)}
}
func TestRoutingFallbackSpecificity(t *testing.T) {
 // Eight different OEM failure phrases observed in nine deduplicated dialogs.
 failures:=[]string{
  "被你问住了，看来要更努力学习了",
  "这个问题我暂时还回答不上，需要再学习一下",
  "这可把我难住了，看来要更努力学习了",
  "这个问题把我难住了，我会继续努力的",
  "这个我暂时还回答不上诶，我要再学习学习",
  "这个问题我还在学习中， 先聊点别的吧",
  "被难住了诶，看来我还要再学习一下",
  "对不起， 我还在学习中!",
 }
 for _,text:=range failures {
  event:=RoutingEvent{Kind:"semantic",Namespace:"Template",Name:"Toast",Task:"new-unknown-id",Text:text}
  if !IsConfirmedFallback(event) {t.Errorf("missed OEM failure: %q",text)}
 }
 // Native Toasts include successful actions, device errors, music, and maths.
 // Even an old fallback task ID is not proof when the text is unrelated.
 native:=[]string{
  "搞定", "正在降低晾衣机。", "好啊。", "好的，搞定了。",
  "正在升高晾衣机。", "正在暂停。", "设备已经关啦",
  "房间温度是22.8度。", "没有发现可以操作的电视，先去米家绑定设备吧。",
  "好嘞，在音乐的世界里，发现美好时光吧",
  "答案是一百九十九", "好啊，为你带来音乐会员专享歌曲",
 }
 for _,text:=range native {
  event:=RoutingEvent{Kind:"semantic",Namespace:"Template",Name:"Toast",Task:"2424b468fff5dfa723ab2b4862f48cfd",Text:text}
  if IsConfirmedFallback(event) {t.Errorf("misclassified native Toast: %q",text)}
 }
 for _,event:=range []RoutingEvent{
  {Kind:"query",Namespace:"Template",Name:"Toast",Text:"我还在学习中"},
  {Kind:"semantic",Namespace:"Template",Name:"General",Text:"我还在学习中"},
  {Kind:"semantic",Namespace:"SpeechSynthesizer",Name:"Speak",Text:"我还在学习中"},
  {Kind:"semantic",Namespace:"Template",Name:"Toast",Text:""},
 } {
  if IsConfirmedFallback(event) {t.Errorf("false fallback: %+v",event)}
 }
}

func TestRoutingRealThirdTaskID(t *testing.T) {
 line:=`11-01 13:30:52.123 I/MICO.cap(42): instruction={"header":{"namespace":"Template","name":"Toast","dialog_id":"abc31"},"payload":{"text":"对不起， 我还在学习中!","display":{"full_screen":{"task":"c21f969b5f03d33d43e04f8f136e7682"}}}}`
 event:=ParseRoutingEvent(line)
 if event.Kind!="semantic" || event.DialogID!="abc31" || !IsConfirmedFallback(event) {
  t.Fatalf("third fallback task missed: %+v",event)
 }
}

func TestRoutingPlaybackCallbacks(t *testing.T) {
 e:=ParseRoutingEvent("D/MICO.speech(27572): onPlayStart=abc31")
 if e.Kind!="play-start" || e.DialogID!="abc31" {t.Fatal(e)}
 if x:=ParseRoutingEvent("D/MICO.speech(27572): onPlayFinish");x.Kind!="play-finish" {t.Fatal(x)}
}
