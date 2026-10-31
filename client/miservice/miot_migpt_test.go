package miservice

import (
 "context"
 "bytes"
 "compress/gzip"
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
func TestMiGPTRC4AllowsEmptyOptionalCUserId(t *testing.T) {
 a:=NewAccount("u","p",nil)
 a.token=NewTokens()
 a.token.UserId="42"
 a.token.DeviceId="device1"
 a.token.Sids[MIIO_SID]=SidToken{
  Ssecurity:base64.StdEncoding.EncodeToString([]byte("test security phrase 123")),
  ServiceToken:"service-test",
 }
 calls:=0
 a.client.Transport=otpTransport(func(req *http.Request)(*http.Response,error){
  calls++
  cookieFound:=false
  for _,c:=range req.Cookies(){if c.Name=="cUserId" {cookieFound=true;if c.Value!="" {t.Fatal("expected empty cUserId")}}}
  if !cookieFound {t.Fatal("missing optional cUserId cookie")}
  return &http.Response{StatusCode:429,Header:make(http.Header),Body:io.NopCloser(strings.NewReader("limited")),Request:req},nil
 })
 _,err:=NewIOService(a,nil).MiotActionMiGPT(context.Background(),"did",5,1,[]interface{}{"test"})
 if err==nil || !strings.Contains(err.Error(),"429") || calls!=1 {t.Fatalf("unexpected result %v calls=%d",err,calls)}
}

func TestMiGPTRC4SuccessfulResponse(t *testing.T) {
 a:=NewAccount("u","p",nil)
 a.token=NewTokens()
 a.token.UserId="42"
 a.token.Sids[MIIO_SID]=SidToken{
  Ssecurity:base64.StdEncoding.EncodeToString([]byte("test security phrase 123")),
  ServiceToken:"service-test",
 }
 a.client.Transport=otpTransport(func(req *http.Request)(*http.Response,error){
  body,err:=io.ReadAll(req.Body);if err!=nil{return nil,err}
  values,err:=url.ParseQuery(string(body));if err!=nil{return nil,err}
  snonce,err:=signNonce(a.token.Sids[MIIO_SID].Ssecurity,values.Get("_nonce"));if err!=nil{return nil,err}
  key,err:=base64.StdEncoding.DecodeString(snonce);if err!=nil{return nil,err}
  encrypted,err:=miGPTRC4(key,[]byte(`{"code":0,"result":{"code":0}}`));if err!=nil{return nil,err}
  return &http.Response{StatusCode:200,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(base64.StdEncoding.EncodeToString(encrypted[0]))),Request:req},nil
 })
 code,err:=NewIOService(a,nil).MiotActionMiGPT(context.Background(),"did",5,1,[]interface{}{"test"})
 if err!=nil || code!=0 {t.Fatalf("successful RC4 reply: code=%v err=%v",code,err)}
}

func TestMiGPTRC4DecodeValidation(t *testing.T) {
 security:=base64.StdEncoding.EncodeToString([]byte("test security phrase 123"))
 nonce:=base64.StdEncoding.EncodeToString([]byte("0123456789ab"))
 snonce,err:=signNonce(security,nonce);if err!=nil{t.Fatal(err)}
 key,err:=base64.StdEncoding.DecodeString(snonce);if err!=nil{t.Fatal(err)}
 encode:=func(payload []byte) []byte {
  t.Helper()
  out,err:=miGPTRC4(key,payload);if err!=nil{t.Fatal(err)}
  return []byte(base64.StdEncoding.EncodeToString(out[0]))
 }
 t.Run("valid JSON",func(t *testing.T){
  value,err:=miGPTDecodeMIoT(security,nonce,encode([]byte(`{"code":0,"result":{"code":0}}`)),false)
  if err!=nil||value["code"]!=float64(0){t.Fatalf("decode failed: %v",err)}
 })
 t.Run("valid gzip",func(t *testing.T){
  var compressed bytes.Buffer
  gz:=gzip.NewWriter(&compressed)
  if _,err:=gz.Write([]byte(`{"code":0,"result":{"code":0}}`));err!=nil{t.Fatal(err)}
  if err:=gz.Close();err!=nil{t.Fatal(err)}
  if _,err:=miGPTDecodeMIoT(security,nonce,encode(compressed.Bytes()),true);err!=nil{t.Fatal(err)}
 })
 t.Run("oversized gzip",func(t *testing.T){
  var compressed bytes.Buffer
  gz:=gzip.NewWriter(&compressed)
  if _,err:=gz.Write(bytes.Repeat([]byte("X"),(1<<20)+1));err!=nil{t.Fatal(err)}
  if err:=gz.Close();err!=nil{t.Fatal(err)}
  if _,err:=miGPTDecodeMIoT(security,nonce,encode(compressed.Bytes()),true);err==nil {t.Fatal("oversized gzip accepted")}
 })
 t.Run("oversized plaintext",func(t *testing.T){
  if _,err:=miGPTDecodeMIoT(security,nonce,encode(bytes.Repeat([]byte("x"),(1<<20)+1)),false);err==nil {t.Fatal("oversized reply accepted")}
 })
}

func TestMiGPTRC4HTTPFailuresDoNotRetry(t *testing.T) {
 for _,tc:=range []struct{status int;message string}{
  {http.StatusUnauthorized,"HTTP 401"},
  {http.StatusForbidden,"HTTP 403"},
  {http.StatusTooManyRequests,"HTTP 429"},
 } {
  t.Run(tc.message,func(t *testing.T){
   a:=NewAccount("u","p",nil)
   a.token=NewTokens()
   a.token.Sids[MIIO_SID]=SidToken{Ssecurity:base64.StdEncoding.EncodeToString([]byte("test secret")),ServiceToken:"token"}
   calls:=0
   a.client.Transport=otpTransport(func(req *http.Request)(*http.Response,error){
    calls++
    return &http.Response{StatusCode:tc.status,Header:http.Header{"Retry-After":[]string{"60"}},Body:io.NopCloser(strings.NewReader("error")),Request:req},nil
   })
   _,err:=NewIOService(a,nil).MiotActionMiGPT(context.Background(),"did",5,1,[]interface{}{"hi"})
   if err==nil||!strings.Contains(err.Error(),tc.message)||calls!=1{t.Fatalf("status=%d calls=%d err=%v",tc.status,calls,err)}
  })
 }
}

func TestMiGPTRC4CancelledBeforeRequest(t *testing.T) {
 a:=NewAccount("u","p",nil)
 a.token=NewTokens()
 a.token.Sids[MIIO_SID]=SidToken{Ssecurity:base64.StdEncoding.EncodeToString([]byte("test secret")),ServiceToken:"token"}
 a.client.Transport=otpTransport(func(req *http.Request)(*http.Response,error){
  t.Fatal("cancelled request reached transport")
  return nil,context.Canceled
 })
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 _,err:=NewIOService(a,nil).MiotActionMiGPT(ctx,"did",5,1,[]interface{}{"hi"})
 if err!=context.Canceled{t.Fatalf("expected cancellation: %v",err)}
}

func TestMiGPTRC4MalformedEncryptedReplies(t *testing.T) {
 for _,tc:=range []struct{name,body string}{
  {"missing top code",`{"result":{"code":0}}`},
  {"wrong top code",`{"code":"0","result":{"code":0}}`},
  {"missing result",`{"code":0}`},
  {"missing action code",`{"code":0,"result":{}}`},
 }{
  t.Run(tc.name,func(t *testing.T){
   a:=NewAccount("u","p",nil)
   a.token=NewTokens()
   a.token.Sids[MIIO_SID]=SidToken{Ssecurity:base64.StdEncoding.EncodeToString([]byte("test secret")),ServiceToken:"token"}
   a.client.Transport=otpTransport(func(req *http.Request)(*http.Response,error){
    body,err:=io.ReadAll(req.Body);if err!=nil{return nil,err}
    vals,err:=url.ParseQuery(string(body));if err!=nil{return nil,err}
    snonce,err:=signNonce(a.token.Sids[MIIO_SID].Ssecurity,vals.Get("_nonce"));if err!=nil{return nil,err}
    key,err:=base64.StdEncoding.DecodeString(snonce);if err!=nil{return nil,err}
    parts,err:=miGPTRC4(key,[]byte(tc.body));if err!=nil{return nil,err}
    return &http.Response{StatusCode:200,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(base64.StdEncoding.EncodeToString(parts[0]))),Request:req},nil
   })
   _,err:=NewIOService(a,nil).MiotActionMiGPT(context.Background(),"did",5,1,[]interface{}{"hi"})
   if err==nil{t.Fatal("malformed response was accepted")}
  })
 }
}

func TestMiGPTRC4ReusesCachedSIDWithoutLogin(t *testing.T) {
 tokens:=NewTokens()
 tokens.UserName="speaker-account"
 tokens.UserId="42"
 tokens.DeviceId="device"
 tokens.Sids[MIIO_SID]=SidToken{Ssecurity:base64.StdEncoding.EncodeToString([]byte("test secret")),ServiceToken:"cached-token"}
 store:=&cachedSIDStore{tokens:tokens}
 a:=NewAccount("speaker-account","unused",store)
 calls:=0
 a.client.Transport=otpTransport(func(req *http.Request)(*http.Response,error){
  calls++
  if req.URL.Host!="api.io.mi.com"{t.Fatalf("unexpected login request: %s",req.URL.Host)}
  return &http.Response{StatusCode:429,Header:make(http.Header),Body:io.NopCloser(strings.NewReader("")),Request:req},nil
 })
 _,err:=NewIOService(a,nil).MiotActionMiGPT(context.Background(),"did",5,1,[]interface{}{"hi"})
 if err==nil||!strings.Contains(err.Error(),"429")||calls!=1||store.saves!=0{t.Fatalf("cache reuse failed: calls=%d saves=%d err=%v",calls,store.saves,err)}
}
