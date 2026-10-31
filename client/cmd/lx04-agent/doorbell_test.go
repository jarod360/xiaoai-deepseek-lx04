package main
import("context";"net/http";"net/http/httptest";"path/filepath";"testing";"xiaobot/devicevoice")
func TestDoorbellGuard(t *testing.T){
 for _,tc:=range []struct{name,body string;status int;blocked bool}{
 {"idle",`{"popups":[]}`,200,false},{"active",`{"popups":[{"id":"doorbell-shared-live"}]}`,200,true},{"historical",`{"popups":[],"lastPopup":{"id":"doorbell-shared-live"}}`,200,false},{"other popup",`{"popups":[{"id":"other"}]}`,200,false},{"missing",`{}`,200,true},{"malformed",`bad`,200,true},{"error",`{"popups":[]}`,500,true},
 }{t.Run(tc.name,func(t *testing.T){srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(tc.status);w.Write([]byte(tc.body))}));defer srv.Close();g:=newDoorbellGuard(srv.URL);if got:=g.check(context.Background());got!=tc.blocked{t.Fatalf("blocked=%v",got)}})}
}
func TestDoorbellCancelsPendingGeneration(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background())
 a:=&agent{gate:&devicevoice.AudioGate{Path:filepath.Join(t.TempDir(),"restore.json")},generation:4,pending:"late answer",cancel:cancel}
 a.preemptForDoorbell()
 if ctx.Err()==nil||a.generation!=5||a.pending!=""{t.Fatal("pending generation not cancelled")}
}