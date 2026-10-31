package miservice

import (
 "context"
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
   if err:=a.Login(MIIO_SID);err!=nil{return -1,err}
 }
 t:=a.token
 sid:=t.Sids[MIIO_SID]
 if sid.Ssecurity==""||sid.ServiceToken=="" {return -1,errors.New("xiaomiio credential incomplete")}
 const path="/miotspec/action"
 nonce:=genNonce()
 if nonce=="" {return -1,errors.New("nonce unavailable")}
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
  return -1,fmt.Errorf("MIoT RC4 HTTP %d",response.StatusCode)
 }
 b,err:=io.ReadAll(io.LimitReader(response.Body,1<<20))
 if err!=nil{return -1,err}
 result,err:=miGPTDecodeMIoT(sid.Ssecurity,nonce,b,response.Header.Get("miot-content-encoding")=="GZIP")
 if err!=nil{return -1,err}
 if code,ok:=result["code"].(float64);ok&&code!=0{return -1,fmt.Errorf("MIoT RC4 API code %.0f",code)}
 action,ok:=result["result"].(map[string]any)
 if !ok{return -1,errors.New("MIoT RC4 result missing")}
 code,ok:=action["code"].(float64)
 if !ok{return -1,errors.New("MIoT RC4 action code missing")}
 return code,nil
}
