package xiaobot

import "testing"

func TestActionIDRejectsMalformedValues(t *testing.T) {
 for _,input:=range []string{"", "5", "5-", "-1", "0-1", "5-0", "-5-1", "5--1", "a-1", "5-xyz", "5-1-extra"} {
  if got:=actionId(input);got!=nil {t.Errorf("actionId(%q)=%v; want nil",input,got)}
 }
 for _,tc:=range []struct{in string;siid,aiid int}{
  {"5-1",5,1},{"7-3",7,3},{" 5 - 1 ",5,1},
 }{
  ids:=actionId(tc.in)
  if len(ids)!=2||ids[0]!=tc.siid||ids[1]!=tc.aiid{t.Errorf("actionId(%q)=%v",tc.in,ids)}
 }
}
