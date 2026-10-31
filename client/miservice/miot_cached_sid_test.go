package miservice

import (
 "encoding/base64"
 "errors"
 "strings"
 "testing"
)
type cachedSIDStore struct { tokens *Tokens; loadErr error; saves int }
func (s *cachedSIDStore) LoadToken() (*Tokens,error) { return s.tokens,s.loadErr }
func (s *cachedSIDStore) SaveToken(*Tokens) error { s.saves++;return nil }
func TestLoadCachedSIDDoesNotLoginOrSave(t *testing.T) {
 tokens:=NewTokens()
 tokens.UserName="speaker-account"
 tokens.Sids[MIIO_SID]=SidToken{Ssecurity:base64.StdEncoding.EncodeToString([]byte("test-secret")),ServiceToken:"test-token"}
 store:=&cachedSIDStore{tokens:tokens}
 account:=NewAccount("speaker-account","unused",store)
 if err:=account.LoadCachedSID(MIIO_SID);err!=nil {t.Fatal(err)}
 if !account.hasSid(MIIO_SID)||store.saves!=0 {t.Fatal("cached SID was not loaded read-only")}
}
func TestLoadCachedSIDFailsClosed(t *testing.T) {
 for _,tc:=range []struct{name string; store *cachedSIDStore; want string}{
  {"not found",&cachedSIDStore{loadErr:errors.New("missing")},"missing"},
  {"wrong account",&cachedSIDStore{tokens:&Tokens{UserName:"different",Sids:map[string]SidToken{}}},"mismatch"},
  {"missing sid",&cachedSIDStore{tokens:&Tokens{UserName:"speaker-account",Sids:map[string]SidToken{}}},"incomplete"},
 } {
  t.Run(tc.name,func(t *testing.T){
   a:=NewAccount("speaker-account","unused",tc.store)
   err:=a.LoadCachedSID(MIIO_SID)
   if err==nil||!strings.Contains(err.Error(),tc.want)||tc.store.saves!=0||a.token!=nil {t.Fatalf("failed closed: %v",err)}
  })
 }
}
