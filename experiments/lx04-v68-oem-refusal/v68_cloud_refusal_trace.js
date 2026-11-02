'use strict';
// LX04 real APK SHA256: 803b6d89ef20df27f563e4cbec109ef7bec1d83c9bdc67990789fc72dd5be71a
// Frida 16.6.6 / arm32. OBSERVE ONLY: no suppression, replay, audio mute, or DeepSeek call.
// V68 exists to explain V67 candidate=0 with privacy-preserving event-level diagnostics.
var V68_MODE = 'OBSERVE';
var V68_WINDOW_MS = 25000;
var V68_MAX_SESSIONS = 32;
var V68_REFUSALS = [
  '被你问住了，看来要更努力学习了',
  '这个问题我暂时还回答不上，需要再学习一下',
  '这可把我难住了，看来要更努力学习了',
  '这个问题把我难住了，我会继续努力的',
  '这个我暂时还回答不上诶，我要再学习学习',
  '这个问题我还在学习中，先聊点别的吧',
  '被难住了诶，看来我还要再学习一下',
  '对不起，我还在学习中!'
];
var V68_NORMALIZE = /[\s，。！？!?,、；;：:"'“”‘’（）()【】\[\]《》…·—-]+/g;
function v68norm(t) {
  var x = String(t === null || t === undefined ? '' : t);
  return (typeof x.normalize === 'function' ? x.normalize('NFKC') : x).replace(V68_NORMALIZE, '');
}
function v68refusalIndex(t) {
  var n = v68norm(t);
  for (var i = 0; i < V68_REFUSALS.length; i++) if (n === v68norm(V68_REFUSALS[i])) return i + 1;
  return 0;
}
var v68Sessions = {};
var v68Order = [];
var v68Installed = [];
var v68Java = null;
var v68ToastClass = null;
var v68SpeakClass = null;
var v68AsrClass = null;
var v68ItemClass = null;
var v68Seq = 0;
var v68Stopped = false;
var v68Counts = {
  cap:0, capWithDialog:0, capNoDialog:0, asrFinal:0, start:0, finish:0,
  toastInstruction:0, speakInstruction:0, nativeConflict:0,
  toastOp:0, speakOp:0, earlyCandidate:0, complete:0, queryMatch:0, refusalKnown:0,
  speakMatchesToast:0, failOpen:0
};
function v68log(s) { console.log(String(s).replace(/[\r\n]/g, ' ')); }
function v68event(s) { v68Seq++; v68log('V68_EVENT seq=' + v68Seq + ' ' + s); }
function v68id(ins) {
  try {
    var opt = ins.getDialogId();
    if (opt && opt.isPresent()) return {present:true, value:String(opt.get())};
  } catch (e) { return {present:false, value:'', error:true}; }
  return {present:false, value:''};
}
function v68alias(id) {
  if (!id) return 'NO_ID';
  if (!v68Sessions[id]) {
    if (v68Order.length >= V68_MAX_SESSIONS) delete v68Sessions[v68Order.shift()];
    v68Order.push(id);
    v68Sessions[id] = {alias:'D' + v68Order.length, asr:'', start:false, finish:false,
      toastCount:0, toastText:'', toastQuery:'', refusalIndex:0, queryMatch:false, speakSeen:false,
      speakText:'', speakMatch:false, earlyCandidate:false, confirmed:false, queryMatchCounted:false, conflict:false};
  }
  return v68Sessions[id].alias;
}
function v68session(id) { return id && v68Sessions[id] ? v68Sessions[id] : null; }
function v68payloadName(ins) {
  try { var p = ins.getPayload(); return p && p.$className ? String(p.$className) : 'PAYLOAD_UNKNOWN'; }
  catch (e) { return 'PAYLOAD_ERROR'; }
}
function v68asrFinal(ins) {
  var payload = v68Java.cast(ins.getPayload(), v68AsrClass);
  if (!payload.isFinal()) return '';
  var items = payload.getResults();
  if (!items || items.size() < 1) return '';
  return String(v68Java.cast(items.get(0), v68ItemClass).getText());
}
function v68nativeConflict(name) {
  return name === 'Template.General' || name === 'Template.WeatherV2' ||
    name === 'Template.DeviceList' || name === 'Template.PlayInfo' ||
    name === 'Nlp.AuxiliaryIntention' || name === 'Dialog.ExitContinuousDialog' ||
    /^(Miot\.|MiotController\.|Device\.|Scene\.|AudioPlayer\.|PlaybackController\.|Alarm\.|Timer\.)/.test(name);
}
function v68evaluate(s) {
  if (!s || s.conflict || s.toastCount !== 1 || !s.toastQuery || !s.toastText || !s.refusalIndex) return;
  var qmatch = !!(s.asr && v68norm(s.toastQuery) === v68norm(s.asr));
  if (qmatch && !s.queryMatchCounted) { s.queryMatchCounted = true; v68Counts.queryMatch++; }
  if (qmatch && s.start && !s.earlyCandidate) {
    s.earlyCandidate = true; v68Counts.earlyCandidate++;
    v68event('EARLY_REFUSAL_CANDIDATE dialog=' + s.alias + ' refusalPattern=' + s.refusalIndex);
  }
  var speakMatch = !!(s.speakSeen && v68norm(s.speakText) === v68norm(s.toastText));
  if (speakMatch && !s.speakMatchCounted) { s.speakMatchCounted = true; v68Counts.speakMatchesToast++; }
  if (qmatch && s.start && s.finish && speakMatch && !s.confirmed) {
    s.confirmed = true; v68Counts.complete++;
    v68event('REFUSAL_CONFIRMED_COMPLETE dialog=' + s.alias + ' refusalPattern=' + s.refusalIndex);
  }
}
function v68revoke(s, reason) {
  if (s && s.confirmed) {
    s.confirmed = false; v68Counts.complete--;
    v68event('REFUSAL_CONFIRMATION_REVOKED dialog=' + s.alias + ' reason=' + reason);
  }
}
function v68mark(ins) {
  var name = String(ins.getFullName());
  var ident = v68id(ins);
  var id = ident.value;
  var alias = v68alias(id);
  var s = v68session(id);
  v68Counts.cap++;
  if (ident.present) v68Counts.capWithDialog++; else v68Counts.capNoDialog++;
  if (!s) {
    v68event('CAP name=' + name + ' dialog=' + alias + ' idPresent=0 payload=' + v68payloadName(ins));
    return;
  }
  if (name === 'SpeechRecognizer.RecognizeResult') {
    var asr = v68asrFinal(ins);
    if (asr) {
      s.asr = asr; v68Counts.asrFinal++;
      v68event('ASR_FINAL dialog=' + alias + ' textLen=' + asr.length);
      v68evaluate(s);
    } else {
      v68event('ASR_NONFINAL dialog=' + alias + ' payload=' + v68payloadName(ins));
    }
    return;
  }
  if (name === 'Nlp.StartAnswer') {
    s.start = true; v68Counts.start++;
    v68event('START_ANSWER dialog=' + alias);
    v68evaluate(s);
    return;
  }
  if (name === 'Nlp.FinishAnswer') {
    s.finish = true; v68Counts.finish++;
    v68event('FINISH_ANSWER dialog=' + alias + ' refusalKnown=' + (s.refusalIndex ? 1 : 0) + ' speakSeen=' + (s.speakSeen ? 1 : 0));
    v68evaluate(s);
    return;
  }
  if (v68nativeConflict(name)) {
    v68revoke(s, 'NATIVE_CONFLICT');
    s.conflict = true; v68Counts.nativeConflict++;
    v68event('NATIVE_CONFLICT dialog=' + alias + ' name=' + name + ' refusalAlreadySeen=' + (s.refusalIndex ? 1 : 0));
    return;
  }
  if (name === 'Template.Toast') {
    var tp = v68Java.cast(ins.getPayload(), v68ToastClass);
    var text = String(tp.getText());
    var qo = tp.getQuery();
    var query = qo && qo.isPresent() ? String(qo.get()) : '';
    var rid = v68refusalIndex(text);
    s.toastCount++;
    if (s.toastCount > 1) v68revoke(s, 'MULTIPLE_TOASTS');
    s.toastText = text; s.toastQuery = query; s.refusalIndex = rid;
    v68Counts.toastInstruction++;
    if (rid) v68Counts.refusalKnown++;
    var qmatch = !!(s.asr && query && v68norm(query) === v68norm(s.asr));
    v68event('TOAST dialog=' + alias + ' toastNo=' + s.toastCount + ' queryPresent=' + (query ? 1 : 0) +
      ' queryLen=' + query.length + ' asrPresent=' + (s.asr ? 1 : 0) + ' asrLen=' + s.asr.length +
      ' queryMatchesAsr=' + (qmatch ? 1 : 0) + ' textLen=' + text.length + ' refusalPattern=' + rid +
      ' startSeen=' + (s.start ? 1 : 0) + ' conflict=' + (s.conflict ? 1 : 0));
    v68evaluate(s);
    return;
  }
  if (name === 'SpeechSynthesizer.Speak') {
    var sp = v68Java.cast(ins.getPayload(), v68SpeakClass);
    var speakText = String(sp.getText());
    var speakMatch = !!(s.refusal && v68norm(speakText) === v68norm(s.refusal));
    s.speakSeen = true; s.speakText = speakText; s.speakMatch = speakMatch;
    v68Counts.speakInstruction++;
    v68event('SPEAK_INSTRUCTION dialog=' + alias + ' textLen=' + speakText.length +
      ' refusalPattern=' + v68refusalIndex(speakText) +
      ' matchesToastAtEvent=' + (speakMatch ? 1 : 0) + ' earlyCandidateAlreadySeen=' + (s.earlyCandidate ? 1 : 0));
    v68evaluate(s);
    return;
  }
  v68event('CAP dialog=' + alias + ' name=' + name + ' payload=' + v68payloadName(ins));
}
function v68op(kind, instance) {
  try {
    var ins = instance.getInstruction();
    var ident = v68id(ins);
    var alias = v68alias(ident.value);
    var name = String(ins.getFullName());
    var extra = '';
    if (name === 'Template.Toast') {
      var p = v68Java.cast(ins.getPayload(), v68ToastClass);
      var q = p.getQuery();
      var qt = q && q.isPresent() ? String(q.get()) : '';
      var tt = String(p.getText());
      extra = ' refusalPattern=' + v68refusalIndex(tt) + ' queryLen=' + qt.length;
    } else if (name === 'SpeechSynthesizer.Speak') {
      var p2 = v68Java.cast(ins.getPayload(), v68SpeakClass);
      extra = ' refusalPattern=' + v68refusalIndex(String(p2.getText()));
    }
    v68event('OP op=' + kind + ' dialog=' + alias + ' name=' + name + ' idPresent=' + (ident.present ? 1 : 0) + extra);
  } catch (e) { v68Counts.failOpen++; v68event('OP_DIAGNOSTIC_ERROR op=' + kind); }
}
function v68detach() {
  if (v68Stopped) return;
  v68Stopped = true;
  function finish() {
    for (var i = v68Installed.length - 1; i >= 0; i--) {
      try { v68Installed[i].implementation = null; }
      catch (e) { v68log('V68_UNHOOK_ERROR ' + String(e).slice(0,100)); }
    }
    v68log('V68_HOOK_REMOVED cap=' + v68Counts.cap + ' capWithDialog=' + v68Counts.capWithDialog +
      ' capNoDialog=' + v68Counts.capNoDialog + ' asrFinal=' + v68Counts.asrFinal +
      ' start=' + v68Counts.start + ' toastInstruction=' + v68Counts.toastInstruction +
      ' speakInstruction=' + v68Counts.speakInstruction + ' finish=' + v68Counts.finish +
      ' nativeConflict=' + v68Counts.nativeConflict + ' toastOp=' + v68Counts.toastOp +
      ' speakOp=' + v68Counts.speakOp + ' earlyCandidate=' + v68Counts.earlyCandidate +
      ' complete=' + v68Counts.complete + ' queryMatch=' + v68Counts.queryMatch + ' refusalKnown=' + v68Counts.refusalKnown +
      ' speakMatchesToast=' + v68Counts.speakMatchesToast + ' failOpen=' + v68Counts.failOpen +
      ' mode=OBSERVE');
  }
  if (v68Java && v68Java.available) v68Java.perform(finish); else finish();
}
function v68install() {
  if (typeof Java === 'undefined' || !Java.available) { v68log('V68_ERROR_JAVA_UNAVAILABLE'); return; }
  v68Java = Java;
  Java.perform(function () {
    try {
      var Cap = Java.use('com.xiaomi.micolauncher.common.speech.capability.InstructionCapabilityImpl');
      var ToastOp = Java.use('com.xiaomi.micolauncher.instruciton.impl.TemplateToastOperation');
      var SpeakOp = Java.use('com.xiaomi.micolauncher.instruciton.impl.SpeechSynthesizerSpeakOperation');
      v68ToastClass = Java.use('com.xiaomi.ai.api.Template$Toast');
      v68SpeakClass = Java.use('com.xiaomi.ai.api.SpeechSynthesizer$Speak');
      v68AsrClass = Java.use('com.xiaomi.ai.api.SpeechRecognizer$RecognizeResult');
      v68ItemClass = Java.use('com.xiaomi.ai.api.SpeechRecognizer$RecognizeResultItem');
      var cap = Cap.process.overload('com.xiaomi.ai.api.common.Instruction');
      var toast = ToastOp.onProcess.overload('boolean');
      var speak = SpeakOp.onProcess.overload('boolean');
      if (cap.returnType.className !== 'boolean') throw Error('CAP_SIGNATURE_MISMATCH');
      v68Counts.toastOp = 0; v68Counts.speakOp = 0;
      toast.implementation = function (dependence) {
        v68Counts.toastOp++; v68op('toast', this);
        return toast.call(this, dependence);
      };
      v68Installed.push(toast);
      speak.implementation = function (dependence) {
        v68Counts.speakOp++; v68op('speak', this);
        return speak.call(this, dependence);
      };
      v68Installed.push(speak);
      cap.implementation = function (ins) {
        try { v68mark(ins); } catch (e) { v68Counts.failOpen++; v68event('CAP_DIAGNOSTIC_ERROR'); }
        return cap.call(this, ins); // Original OEM dispatch exactly once.
      };
      v68Installed.push(cap);
      v68log('V68_HOOK_ASSIGNED mode=OBSERVE only; original methods remain enabled');
      setTimeout(v68detach, V68_WINDOW_MS);
    } catch (e) { v68log('V68_INSTALL_ABORT ' + String(e).slice(0,180)); v68detach(); }
  });
}
if (typeof globalThis !== 'undefined' && globalThis.__V68_MOCK__ === true) {
  globalThis.__V68_TEST__ = {norm:v68norm, refusalIndex:v68refusalIndex, evaluate:v68evaluate, revoke:v68revoke, counts:v68Counts, sessions:v68Sessions, order:v68Order, createSession:function(id){v68alias(id);return v68session(id);}};
} else if (typeof setImmediate === 'function') setImmediate(v68install); else v68install();