// SPDX-License-Identifier: Apache-2.0
package main
import("context";"encoding/json";"errors";"io";"net/http";"sync";"time")
type doorbellGuard struct{mu sync.Mutex;url string;client *http.Client;blocked bool;holdUntil time.Time}
func newDoorbellGuard(url string)*doorbellGuard{return &doorbellGuard{url:url,client:&http.Client{Timeout:500*time.Millisecond},blocked:true}}
func(g *doorbellGuard)check(ctx context.Context)bool{
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,g.url,nil);active:=false
 if err==nil{var res *http.Response;res,err=g.client.Do(req);if err==nil{
 if res.StatusCode!=http.StatusOK{err=errors.New("receiver status unavailable")}else{
 var body struct{Popups *[]struct{ID string `json:"id"`} `json:"popups"`}
 err=json.NewDecoder(io.LimitReader(res.Body,1024*1024)).Decode(&body)
 if err==nil&&body.Popups==nil{err=errors.New("receiver state missing")}
 if err==nil{for _,p:=range *body.Popups{if p.ID=="doorbell-shared-live"{active=true}}}
 };res.Body.Close()}}
 g.mu.Lock();defer g.mu.Unlock()
 if active{g.holdUntil=time.Now().Add(8*time.Second)}
 g.blocked=err!=nil||active||time.Now().Before(g.holdUntil);return g.blocked
}
func(g *doorbellGuard)watch(ctx context.Context,preempt func()){
 timer:=time.NewTicker(250*time.Millisecond);defer timer.Stop();was:=false
 for{blocked:=g.check(ctx);if blocked&&!was{preempt()};was=blocked
 select{case <-ctx.Done():return;case <-timer.C:}}
}