package miservice

import (
 "context"
 "encoding/base64"
 "encoding/json"
 "io"
 "net/http"
 "net/url"
 "strings"
 "testing"
)
func TestMiGPTRC4RequestAnd429NoRetry(t *testing.T) {
 a:=NewAccount("u","p",nil)
 a.token=NewTokens()
 a.token.UserId="42"
 a.token.CUserId="encrypted-user"
 a.token.DeviceId="device1"
 a.token.Sids[MIIO_SID]=SidToken{
   Ssecurity:base64.StdEncoding.EncodeToString([]byte("test security phrase 123")),
   ServiceToken:"service-test",
 }
 calls:=0
 a.client.Transport=otpTransport(func(req *http.Request)(*http.Response,error){
   calls++
   if req.Method!="POST" || req.URL.Path!="/app/miotspec/action" {t.Fatalf("unexpected endpoint: %s",req.URL)}
   if req.Header.Get("miot-encrypt-algorithm")!="ENCRYPT-RC4" || req.Header.Get("miot-accept-encoding")!="GZIP" {t.Fatal("missing encryption headers")}
   expected:=map[string]string{"cUserId":"encrypted-user","PassportDeviceId":"device1","yetAnotherServiceToken":"service-test","timezone_id":"Asia/Shanghai"}
   for _,c:=range req.Cookies(){if v,ok:=expected[c.Name];ok {if c.Value!=v {t.Fatalf("wrong cookie %s",c.Name)};delete(expected,c.Name)}}
   if len(expected)!=0 {t.Fatalf("missing cookies: %v",expected)}
   body,err:=io.ReadAll(req.Body);if err!=nil{t.Fatal(err)}
   v,err:=url.ParseQuery(string(body));if err!=nil{t.Fatal(err)}
   if len(v["data"])!=1||len(v["rc4_hash__"])!=1||len(v["signature"])!=1||len(v["ssecurity"])!=1{t.Fatalf("incomplete RC4 form")}
   nonce:=v.Get("_nonce");snonce,err:=signNonce(a.token.Sids[MIIO_SID].Ssecurity,nonce);if err!=nil{t.Fatal(err)}
   key,_:=base64.StdEncoding.DecodeString(snonce)
   encData,_:=base64.StdEncoding.DecodeString(v.Get("data"))
   encHash,_:=base64.StdEncoding.DecodeString(v.Get("rc4_hash__"))
   decoded,err:=miGPTRC4(key,encData,encHash);if err!=nil{t.Fatal(err)}
   var payload map[string]any
   if json.Unmarshal(decoded[0],&payload)!=nil || payload["datasource"]!=float64(2) {t.Fatalf("missing datasource=2")}
   if string(decoded[1])!=miGPTSHA1("POST","/miotspec/action","data="+string(decoded[0]),snonce) {t.Fatal("invalid inner hash")}
   if v.Get("signature")!=miGPTSHA1("POST","/miotspec/action","data="+v.Get("data"),"rc4_hash__="+v.Get("rc4_hash__"),snonce){t.Fatal("invalid outer signature")}
   return &http.Response{StatusCode:429,Header:make(http.Header),Body:io.NopCloser(strings.NewReader("limited")),Request:req},nil
 })
 service:=NewIOService(a,nil)
 _,err:=service.MiotActionMiGPT(context.Background(),"did1",5,1,[]interface{}{"hello"})
 if err==nil||!strings.Contains(err.Error(),"429")||calls!=1 {t.Fatalf("expected one HTTP429 without retry, got %v calls=%d",err,calls)}
}
func TestMiGPTRC4MissingCUserIdDoesNotSend(t *testing.T) {
 a:=NewAccount("u","p",nil)
 a.token=NewTokens()
 a.token.Sids[MIIO_SID]=SidToken{Ssecurity:"a",ServiceToken:"b"}
 a.client.Transport=otpTransport(func(r *http.Request)(*http.Response,error){t.Fatal("unexpected network call");return nil,nil})
 _,err:=NewIOService(a,nil).MiotActionMiGPT(context.Background(),"did",5,1,nil)
 if err==nil||!strings.Contains(err.Error(),"cUserId") {t.Fatal(err)}
}
