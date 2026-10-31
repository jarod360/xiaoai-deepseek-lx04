package main
import("context";"testing";"time")
func TestSpeechEOFRetriesWithoutCancellingAgent(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 reads,recovered:=0,0
 retrySpeech(ctx,func(c context.Context){if c.Err()!=nil{t.Fatal("cancelled before reconnect")};reads++;if reads==2{cancel()}},func(){recovered++},time.Millisecond)
 if reads!=2||recovered!=1{t.Fatalf("reads=%d recovered=%d",reads,recovered)}
}
func TestSpeechShutdownInterruptsReconnectWait(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background());done:=make(chan struct{})
 go func(){retrySpeech(ctx,func(context.Context){},cancel,time.Hour);close(done)}()
 select{case <-done:case <-time.After(time.Second):t.Fatal("shutdown blocked")}
}