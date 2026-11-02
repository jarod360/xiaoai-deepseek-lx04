'use strict';
// LX04 real OEM APK only: SHA256 803b6d89ef20df27f563e4cbec109ef7bec1d83c9bdc67990789fc72dd5be71a
// Frida 16.6.6 / Android 8.1 ARM32. Manual injection ONLY after ownership/PID safety preflight.
// FALSE = transparent observation. TRUE = EXPERIMENTAL batch filtering of confirmed refusal
// Toast + Speak ONLY. Does NOT block earlier PCM; NOT production-safe zero-speech fallback.
var V66_EXPERIMENTAL_FILTER = false;
var V66_REFUSALS = [
  '被你问住了，看来要更努力学习了',
  '这个问题我暂时还回答不上，需要再学习一下',
  '这可把我难住了，看来要更努力学习了',
  '这个问题把我难住了，我会继续努力的',
  '这个我暂时还回答不上诶，我要再学习学习',
  '这个问题我还在学习中，先聊点别的吧',
  '被难住了诶，看来我还要再学习一下',
  '对不起，我还在学习中!'
];
function v66Norm(s) {
  return String(s == null ? '' : s).normalize('NFKC')
    .replace(/[\s，。！？!?,、；;：:"'“”‘’（）()【】\[\]《》…·—-]+/g, '');
}
var V66_WHITELIST = V66_REFUSALS.map(v66Norm);
function v66IsConflict(name) {
  if (name === 'Template.General' || name === 'Template.WeatherV2' ||
      name === 'Template.DeviceList' || name === 'Template.PlayInfo' ||
      name === 'Dialog.ExitContinuousDialog' || name === 'Nlp.AuxiliaryIntention') return true;
  return /^(Miot\.|MiotController\.|Device\.|Scene\.|AudioPlayer\.|PlaybackController\.|Alarm\.|Timer\.)/.test(name);
}
function v66Classify(s, batchNames) {
  if (!s || !s.final || !s.start || !s.finish || s.conflict ||
      s.toastCount !== 1 || s.speakCount !== 1 || !s.asr || !s.query ||
      v66Norm(s.asr) !== v66Norm(s.query) || !s.toast || !s.speak ||
      v66Norm(s.toast) !== v66Norm(s.speak) ||
      V66_WHITELIST.indexOf(v66Norm(s.toast)) < 0) return false;
  // Batch interception only makes sense when BOTH operations are in this batch.
  return batchNames.filter(function (x) { return x === 'Template.Toast'; }).length === 1 &&
         batchNames.filter(function (x) { return x === 'SpeechSynthesizer.Speak'; }).length === 1 &&
         !batchNames.some(v66IsConflict);
}
function v66Run() {
  if (typeof Java === 'undefined' || !Java.available) { console.log('V66_ERROR_JAVA_UNAVAILABLE'); return; }
  Java.perform(function () {
    var cap = null, op = null;
    try {
      var Capability = Java.use('com.xiaomi.micolauncher.common.speech.capability.InstructionCapabilityImpl');
      var Manager = Java.use('com.xiaomi.micolauncher.instruciton.base.OperationManager');
      var Recognize = Java.use('com.xiaomi.ai.api.SpeechRecognizer$RecognizeResult');
      var Item = Java.use('com.xiaomi.ai.api.SpeechRecognizer$RecognizeResultItem');
      var Toast = Java.use('com.xiaomi.ai.api.Template$Toast');
      var Speak = Java.use('com.xiaomi.ai.api.SpeechSynthesizer$Speak');
      var List = Java.use('java.util.List');
      var ArrayList = Java.use('java.util.ArrayList');
      cap = Capability.process.overload('com.xiaomi.ai.api.common.Instruction');
      op = Manager.addInstructions.overload('java.lang.String', 'java.util.List');
      if (cap.returnType.className !== 'boolean' || op.returnType.className !== 'void') {
        console.log('V66_ERROR_SIGNATURE_MISMATCH'); return;
      }
      var states = Object.create(null), aliases = Object.create(null), count = 0;
      function alias(id) {
        if (!Object.prototype.hasOwnProperty.call(aliases,id)) aliases[id] = 'dialog_' + (++count);
        return aliases[id];
      }
      function getId(ins) {
        var optional = ins.getDialogId();
        return optional && optional.isPresent() ? String(optional.get()) : '';
      }
      function getState(id) {
        if (!states[id]) states[id] = {
          final:false, start:false, finish:false, conflict:false,
          asr:'', query:'', toast:'', speak:'', toastCount:0, speakCount:0
        };
        return states[id];
      }
      var procCalls = 0, batchCalls = 0, candidates = 0;
      function inspect(ins) {
        var name = String(ins.getFullName());
        var id = getId(ins);
        if (!id) return;
        var state = getState(id);
        if (v66IsConflict(name)) { state.conflict = true; return; }
        if (name === 'Nlp.StartAnswer') { state.start = true; return; }
        if (name === 'Nlp.FinishAnswer') { state.finish = true; return; }
        if (name === 'SpeechRecognizer.RecognizeResult') {
          var r = Java.cast(ins.getPayload(), Recognize);
          if (r.isFinal()) {
            var results = r.getResults();
            if (results && results.size() === 1) {
              state.asr = String(Java.cast(results.get(0),Item).getText());
              state.final = !!v66Norm(state.asr);
            }
          }
        } else if (name === 'Template.Toast') {
          state.toastCount++;
          var toast = Java.cast(ins.getPayload(), Toast);
          state.toast = String(toast.getText());
          var q = toast.getQuery();
          state.query = q && q.isPresent() ? String(q.get()) : '';
        } else if (name === 'SpeechSynthesizer.Speak') {
          state.speakCount++;
          state.speak = String(Java.cast(ins.getPayload(), Speak).getText());
        }
      }
      cap.implementation = function (ins) {
        procCalls++;
        try { inspect(ins); } catch (err) {
          console.log('V66_INSPECTION_ERROR; passthrough=' + String(err).slice(0,96));
        }
        return cap.call(this,ins);
      };
      op.implementation = function (id,list) {
        batchCalls++;
        var chosen = list, decision = false, names = [];
        try {
          var dialogId = String(id || '');
          var length = list ? Number(list.size()) : 0;
          if (dialogId && length > 0 && length <= 128) {
            for (var i=0; i<length; i++) names.push(String(list.get(i).getFullName()));
            decision = v66Classify(states[dialogId],names);
            if (decision) candidates++;
            // This is an optional experimental operation-layer filter, NOT a PCM gate.
            if (decision && V66_EXPERIMENTAL_FILTER) {
              var copy = ArrayList.$new();
              for (var j=0; j<length; j++) {
                if (names[j] !== 'Template.Toast' && names[j] !== 'SpeechSynthesizer.Speak')
                  copy.add(list.get(j));
              }
              chosen = copy;
            }
            console.log('V66_BATCH_OBS ' + JSON.stringify({
              dialog:alias(dialogId), call:batchCalls, items:length,
              types:names, refusal:decision, filtered:chosen!==list
            }));
          } else console.log('V66_BATCH_UNVERIFIABLE; passthrough=1');
        } catch(err) {
          // Never alter a batch on an inspection error.
          chosen = list;
          console.log('V66_BATCH_ERROR; passthrough=' + String(err).slice(0,96));
        }
        return op.call(this,id,chosen);
      };
      console.log('V66_HOOK_ASSIGNED mode=' + (V66_EXPERIMENTAL_FILTER?'EXPERIMENTAL_BATCH_FILTER':'OBSERVE') +
                  ' process=boolean addInstructions=void; NOT A HOOK HIT; NO PCM BLOCK');
      // No auto-administration of Frida Server: only the separate existing owned-session runner may detach.
    } catch (e) {
      try { if (cap) cap.implementation = null; } catch(_) {}
      try { if (op) op.implementation = null; } catch(_) {}
      console.log('V66_INSTALL_FAILED ' + String(e).slice(0,128));
    }
  });
}
v66Run();