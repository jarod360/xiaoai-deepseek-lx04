// SPDX-License-Identifier: Apache-2.0
package devicevoice

import (
 "encoding/json"
 "strings"
)

type RoutingEvent struct {
 Kind,DialogID,Text,Namespace,Name,Task string
}
func ParseRoutingEvent(line string) RoutingEvent {
 if strings.Contains(line,"MICO.speech") && strings.Contains(line,"onPlayStart=") {
  i:=strings.Index(line,"onPlayStart=")+len("onPlayStart=")
  words:=strings.Fields(line[i:])
  if len(words)==0 || words[0]=="" {return RoutingEvent{}}
  return RoutingEvent{Kind:"play-start",DialogID:words[0]}
 }
 if strings.Contains(line,"MICO.speech") && strings.Contains(line,"onPlayFinish") {
  return RoutingEvent{Kind:"play-finish"}
 }
 if !strings.Contains(line,"MICO.cap") {return RoutingEvent{}}
 i:=strings.Index(line,"instruction=")
 if i<0 {return RoutingEvent{}}
 var v struct {
  Header struct {
   Namespace string `json:"namespace"`
   Name string `json:"name"`
   DialogID string `json:"dialog_id"`
  } `json:"header"`
  Payload struct {
   Final bool `json:"is_final"`
   Results []struct {Text string `json:"text"`} `json:"results"`
   Text string `json:"text"`
   Display struct {
    FullScreen struct {Task string `json:"task"`} `json:"full_screen"`
   } `json:"display"`
  } `json:"payload"`
 }
 if json.Unmarshal([]byte(line[i+len("instruction="):]),&v)!=nil {return RoutingEvent{}}
 e:=RoutingEvent{DialogID:v.Header.DialogID,Namespace:v.Header.Namespace,Name:v.Header.Name,Text:v.Payload.Text,Task:v.Payload.Display.FullScreen.Task}
 switch {
 case e.Namespace=="SpeechRecognizer" && e.Name=="RecognizeResult" && v.Payload.Final && len(v.Payload.Results)>0 && e.DialogID!="":
  e.Kind="query";e.Text=v.Payload.Results[0].Text
 case e.Namespace=="SpeechSynthesizer" && e.Name=="Speak":e.Kind="speak"
 case e.Namespace=="Template",e.Namespace=="PlaybackController",(e.Namespace=="Dialog"&&e.Name=="ExitContinuousDialog"):e.Kind="semantic"
 }
 return e
}
// These task IDs represent observed fallback candidates, not universal semantics.
func IsConfirmedFallback(e RoutingEvent) bool {
 return e.Kind=="semantic" && e.Namespace=="Template" && e.Name=="Toast" &&
  (e.Task=="2424b468fff5dfa723ab2b4862f48cfd" || e.Task=="43234f6eb43a7e551ebc1f57a5b5bc75")
}
