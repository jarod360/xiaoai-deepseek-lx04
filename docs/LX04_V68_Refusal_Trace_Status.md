# LX04 V68 — OEM Cloud Refusal Trace

## Goal
Preserve OEM wake/ASR and cloud-first behavior. Only a confirmed OEM cloud refusal may later be suppressed per dialog and handed to the existing DeepSeek path. Do not rework already verified DeepSeek TTS/full-screen/scrolling, V57 ASR-only, or V58 OEM NLP time announcement.

## V67 real-device result (2026-11-02)

```text
V67_HOOK_COUNTS cap=16 toastOp=0 speakOp=1 candidates=0 toastBlocked=0 speakBlocked=0 failOpen=0
V67_FRIDA_CLIENT_EXIT=0
V67_OWNED_SERVER_CLEANUP_ERROR=COMMAND_EXIT_6: device_v67_cleanup.sh: awk: not found
V67_OEM_AFTER_PID=13551 V67_OEM_AFTER_TRACER_PID=0
V67_NOT_PASSED_NO_AUTO_RETRY=1
```

Interpretation:
- `cap=16` proves the real OEM `InstructionCapabilityImpl.process(Instruction)` hook was invoked 16 times.
- `speakOp=1` proves the real OEM Speak operation method was invoked once.
- `toastOp=0` and `candidate=0` mean refusal correlation is not yet proven. V67 did not log enough intermediate fields to identify the failed condition.
- No text/audio was suppressed; no DeepSeek handoff occurred.
- Cleanup failed before TERM because the Android shell had no `awk`; the V67-owned server may remain. V68 cleanup verifies the marker, server command line and binary SHA before sending TERM to that one server only.

## V68 implementation

- `v68_cloud_refusal_trace.js`: observe-only Frida probe. Logs CAP instruction names, dialog presence (aliases only), final-ASR presence/length, Start/FinishAnswer, Toast query/ASR match, refusal whitelist index, Speak/Toast match, native conflict and actual Toast/Speak operation entry.
- Event evaluation is order-independent: Toast may arrive before ASR or StartAnswer; the same dialog is re-evaluated as later events arrive. Complete confirmation requires ASR + StartAnswer + FinishAnswer + one exact refusal Toast + exact query match + matching Speak + no native conflict.
- No raw user query, refusal sentence or real dialog ID is emitted.
- `device_v68_cleanup.sh` avoids awk, validates owned marker, exact server command line, server SHA256, unique OEM PID and TracerPid, then sends TERM only to the owned Frida server. It has no SIGKILL fallback.
- `02_V68_OEM_Cloud_Observe_Once.ps1` attaches once for 25 seconds, preserves original OEM method calls and auto-cleans the owned server.

## Local checks

- `node --check v68_cloud_refusal_trace.js`: passed.
- `sh -n device_v68_cleanup.sh`: passed.
- PowerShell AST check must be run on the user's Windows machine.

## Device test sequence

1. Run `03_V68_Syntax_Check.ps1`.
2. Run `00_V68_Cleanup_Existing_Server.ps1`; require `V68_CLEANUP_PASS=1`, OEM PID unchanged, TracerPid 0, marker absent and Frida process count 0.
3. Run `02_V68_OEM_Cloud_Observe_Once.ps1`; say one previously verified harmless refusal query after the prompt.
4. Upload only the generated `report.txt`; keep PRIVATE Frida stdout/stderr on D:.

## Acceptance boundaries

V68 trace PASS is not refusal-classification PASS. `complete>0` means the event evidence matched the offline classifier, not that output was suppressed. Text/audio suppression and DeepSeek handoff remain unimplemented until per-dialog PCM gating and normal OEM skill regression are verified.


## V68.1 offline correction (2026-11-02)

- A Node order test caught and fixed a case-sensitive event-sequence counter typo before another device run.
- The V68 aggregator now reevaluates one dialog as ASR, StartAnswer, Toast, Speak and FinishAnswer arrive in any order. It requires one exact whitelist Toast, exact Toast.query-to-ASR match, matching Speak, Start/FinishAnswer, and no native-skill conflict.
- Multiple Toasts or a native conflict revoke a previously complete classification. Unknown Toast text, query mismatch, or Speak mismatch remain fail-open.
- test_v68_classifier_order.js passed 10 checks, including Toast-before-ASR/StartAnswer, unknown text, query mismatch, Speak mismatch, multiple Toasts and native conflict.
- node --check and sh -n passed locally. Windows PowerShell AST test remains to be run by the user.
- This remains OBSERVE only. No OEM instruction suppression, audio gating, ASR rewrite, NLP replay or DeepSeek handoff has been implemented or tested on-device.
