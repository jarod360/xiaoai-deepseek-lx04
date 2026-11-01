package main

import "testing"

func TestWakeupFocusOnlyWhenCurrent(t *testing.T) {
 cases:=[]struct{log string;want bool}{
  {"mCurrentFocus=Window{a Sys2038:com.xiaomi.micolauncher/com.xiaomi.micolauncher.Launcher}",true},
  {"mCurrentFocus=Window{b com.xiaomi.micolauncher/com.xiaomi.micolauncher.skills.baike.view.BaikeTextActivity}",false},
  {"Window #0 Sys2038:com.xiaomi.micolauncher/com.xiaomi.micolauncher.Launcher\nmCurrentFocus=Window{b com.other/.Activity}",false},
  {"mCurrentFocus=null",false},
  {"",false},
 }
 for _,tc:=range cases {
  if got:=hasWakeupFocus(tc.log);got!=tc.want {t.Errorf("focus %q got %t want %t",tc.log,got,tc.want)}
 }
}
