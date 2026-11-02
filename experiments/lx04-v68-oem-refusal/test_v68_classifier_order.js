'use strict';
// Offline-only test of V68 order-independent refusal evidence aggregation.
const fs = require('fs');
const vm = require('vm');
const source = fs.readFileSync(__dirname + '/v68_cloud_refusal_trace.js', 'utf8');
const context = {__V68_MOCK__: true, console: {log: function () {}}};
vm.createContext(context);
vm.runInContext(source, context, {filename: 'v68_cloud_refusal_trace.js'});
const api = context.__V68_TEST__;
if (!api) throw new Error('V68_TEST_EXPORT_MISSING');
const refusal = '被你问住了，看来要更努力学习了';
const query = '介绍一下延迟器';
function reset() {
  for (const k of Object.keys(api.sessions)) delete api.sessions[k];
  api.order.length = 0;
  for (const k of Object.keys(api.counts)) api.counts[k] = 0;
}
function session() { return api.createSession('opaque-dialog-id'); }
function assert(ok, label) { if (!ok) throw new Error('FAIL ' + label); console.log('PASS ' + label); }
function addToast(s, q=query, t=refusal) {
  s.toastCount = 1; s.toastText = t; s.toastQuery = q; s.refusalIndex = api.refusalIndex(t);
  api.evaluate(s);
}
function addAsr(s, q=query) { s.asr = q; api.evaluate(s); }
function addStart(s) { s.start = true; api.evaluate(s); }
function addSpeak(s, t=refusal) { s.speakSeen = true; s.speakText = t; api.evaluate(s); }
function addFinish(s) { s.finish = true; api.evaluate(s); }

// Out-of-order cloud arrival: Toast first, then ASR, StartAnswer, Speak, FinishAnswer.
reset();
let s = session(); addToast(s); addAsr(s); assert(api.counts.earlyCandidate === 0, 'no early candidate before StartAnswer');
addStart(s); assert(api.counts.earlyCandidate === 1, 'early candidate after late StartAnswer');
addSpeak(s); assert(api.counts.complete === 0, 'not complete before FinishAnswer');
addFinish(s); assert(api.counts.complete === 1, 'complete refusal after all same-dialog evidence');

// Normal response / unknown refusal phrase must fail open.
reset(); s = session(); addToast(s, query, '好的，我来帮你查询'); addAsr(s); addStart(s); addSpeak(s, '好的，我来帮你查询'); addFinish(s);
assert(api.counts.complete === 0 && api.counts.earlyCandidate === 0, 'unknown Toast text never classified as refusal');

// Query mismatch must fail open.
reset(); s = session(); addToast(s, '另一个问题', refusal); addAsr(s, query); addStart(s); addSpeak(s, refusal); addFinish(s);
assert(api.counts.complete === 0, 'Toast.query mismatch never classified as refusal');

// Speak mismatch must fail open.
reset(); s = session(); addToast(s); addAsr(s); addStart(s); addSpeak(s, '另一段语音'); addFinish(s);
assert(api.counts.complete === 0, 'Speak text mismatch never classified as refusal');

// A second Toast revokes a previously complete candidate.
reset(); s = session(); addToast(s); addAsr(s); addStart(s); addSpeak(s); addFinish(s);
assert(api.counts.complete === 1, 'baseline complete before conflict');
s.toastCount = 2; api.revoke(s, 'MULTIPLE_TOASTS'); api.evaluate(s);
assert(api.counts.complete === 0, 'multiple Toast revokes complete classification');

// Native conflict revokes a complete refusal.
reset(); s = session(); addToast(s); addAsr(s); addStart(s); addSpeak(s); addFinish(s);
s.conflict = true; api.revoke(s, 'NATIVE_CONFLICT'); api.evaluate(s);
assert(api.counts.complete === 0, 'native skill conflict revokes refusal');

console.log('V68_CLASSIFIER_ORDER_TEST_PASS=1');