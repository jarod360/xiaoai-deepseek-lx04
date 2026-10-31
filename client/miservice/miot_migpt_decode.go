package miservice

import (
 "bytes"
 "compress/gzip"
 "crypto/rc4"
 "encoding/base64"
 "encoding/json"
 "errors"
 "io"
)
func miGPTDecodeMIoT(ssecurity, nonce string, encoded []byte, zipped bool) (map[string]any,error) {
 snonce, err := signNonce(ssecurity, nonce)
 if err!=nil {return nil,err}
 key,err:=base64.StdEncoding.DecodeString(snonce)
 if err!=nil {return nil,err}
 ciphertext,err:=base64.StdEncoding.DecodeString(string(bytes.TrimSpace(encoded)))
 if err!=nil {return nil,errors.New("RC4 reply is not base64")}
 cipher,err:=rc4.NewCipher(key)
 if err!=nil {return nil,err}
 discard:=make([]byte,1024)
 cipher.XORKeyStream(discard,discard)
 cipher.XORKeyStream(ciphertext,ciphertext)
 if zipped {
   zr,e:=gzip.NewReader(bytes.NewReader(ciphertext))
   if e!=nil {return nil,errors.New("RC4 reply invalid gzip")}
   defer zr.Close()
   ciphertext,err=io.ReadAll(io.LimitReader(zr,1<<20))
   if err!=nil {return nil,err}
 }
 var res map[string]any
 if json.Unmarshal(ciphertext,&res)!=nil {return nil,errors.New("RC4 reply invalid JSON")}
 return res,nil
}
