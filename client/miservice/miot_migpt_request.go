package miservice

import (
 "context"
 "crypto/rand"
 "encoding/base64"
 "errors"
 "fmt"
 "io"
 "net/http"
 "strings"
)

func (s *IOService) MiotActionMiGPT(ctx context.Context, did string, siid, aiid int, args []interface{}) (float64,error) {
 if err:=ctx.Err();err!=nil{return -1,err}
 a:=s.account
 if !a.hasSid(MIIO_SID) {
   // Prefer a complete cached SID before attempting another Xiaomi login.
   if err:=a.LoadCachedSID(MIIO_SID);err!=nil {
     if err:=ctx.Err();err!=nil{return -1,err}
     if err:=a.Login(MIIO_SID);err!=nil{return -1,err}
   }
 }
 t:=a.token
 sid:=t.Sids[MIIO_SID]
 if sid.Ssecurity==""||sid.ServiceToken=="" {return -1,errors.New("xiaomiio credential incomplete")}
 const path="/miotspec/action"
 nonceBytes:=make([]byte,12)
 if _,err:=rand.Read(nonceBytes);err!=nil {return -1,errors.New("nonce unavailable")}
 nonce:=base64.StdEncoding.EncodeToString(nonceBytes)
 data,err:=miGPTEncodeMIoT(path,map[string]any{"did":did,"siid":siid,"aiid":aiid,"in":args},sid.Ssecurity,nonce)
 if err!=nil{return -1,err}
 req,err:=http.NewRequestWithContext(ctx,"POST",s.server+path,strings.NewReader(data.Encode()))
 if err!=nil{return -1,err}
 req.Header.Set("Content-Type","application/x-www-form-urlencoded")
 req.Header.Set("User-Agent","MICO/AndroidApp/@SHIP.TO.2A2FE0D7@/2.4.40")
 req.Header.Set("x-xiaomi-protocal-flag-cli","PROTOCAL-HTTP2")
 req.Header.Set("miot-accept-encoding","GZIP")
 req.Header.Set("miot-encrypt-algorithm","ENCRYPT-RC4")
 for _,p:=range [][2]string{
  {"countryCode","CN"},{"locale","zh_CN"},{"timezone","GMT+08:00"},{"timezone_id","Asia/Shanghai"},
  {"userId",t.UserId},{"cUserId",t.CUserId},{"PassportDeviceId",t.DeviceId},
  {"serviceToken",sid.ServiceToken},{"yetAnotherServiceToken",sid.ServiceToken},
 } {req.AddCookie(&http.Cookie{Name:p[0],Value:p[1]})}
 client:=*a.client
 client.Jar=nil
 response,err:=client.Do(req)
 if err!=nil{return -1,err}
 defer response.Body.Close()
 if response.StatusCode!=200 {
  if response.StatusCode==429 {
   retryAfter:=strings.TrimSpace(response.Header.Get("Retry-After"))
   if len(retryAfter)>80 {retryAfter=retryAfter[:80]}
   if retryAfter=="" {retryAfter="absent"}
   return -1,fmt.Errorf("MIoT RC4 HTTP 429 (Retry-After %q)",retryAfter)
  }
  switch response.StatusCode {
   case http.StatusUnauthorized:
    return -1,fmt.Errorf("MIoT RC4 authentication rejected (HTTP 401; no automatic login or retry)")
   case http.StatusForbidden:
    return -1,fmt.Errorf("MIoT RC4 access forbidden (HTTP 403; no automatic login or retry)")
   default:
    return -1,fmt.Errorf("MIoT RC4 HTTP %d",response.StatusCode)
  }
 }
 b,err:=io.ReadAll(io.LimitReader(response.Body,1<<20))
 if err!=nil{return -1,err}
 result,err:=miGPTDecodeMIoT(sid.Ssecurity,nonce,b,strings.EqualFold(strings.TrimSpace(response.Header.Get("miot-content-encoding")),"GZIP"))
 if err!=nil{return -1,err}
 apiCode,ok:=result["code"].(float64)
 if !ok{return -1,errors.New("MIoT RC4 API code missing")}
 if apiCode!=0{return -1,fmt.Errorf("MIoT RC4 API code %.0f",apiCode)}
 action,ok:=result["result"].(map[string]any)
 if !ok{return -1,errors.New("MIoT RC4 result missing")}
 code,ok:=action["code"].(float64)
 if !ok{return -1,errors.New("MIoT RC4 action code missing")}
 return code,nil
}
