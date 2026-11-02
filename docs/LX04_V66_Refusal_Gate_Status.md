# XiaoSeek LX04｜最新研究与开发状态 V66（2026-11-02）

> 新会话 **先读本文**。该文件是当前 GitHub V66 研究分支的权威进度页；长历史以之前的 V65 完整交接 MD 为准。文中严格区分实机、离线、假设。不得用旧 Github 分支代码冒充设备当前二进制。

## 0. 唯一目标（用户反复确认）

**原厂优先、只有原厂云端明确拒答才将本轮问题交给 DeepSeek**。必须在原厂拒答文字和音频输出前截住对应会话；不抑制报时、天气、设备/米家、场景、音乐、正常答复。必须使用原厂真实云端结构化指令与 `dialog_id`，不能依据 ASR 关键词或超时猜测。目标是零原厂拒答残留文字、零原厂拒答可听发音、DeepSeek 单次接管、下一轮唤醒正常；旧版的事后 TTS stop、全局 master mute/BACK 不作为解决该问题的方法。

## 1. 已实机实现，严禁重做

- DeepSeek 回答、原厂 TTS 播报、BaikeTextActivity 全屏文字、长文滚动：已工作。
- uinput 底部条关闭：已有实机方案，但偶发干扰仍需回归。
- V55 透明 Hook；V57 单轮 Recognize `RequestControl.disabled=[TTS,NLP]` 后保留 final ASR、同轮无原生 NLP/TTS；V58 `SpeechManager.nlpTtsRequest("现在几点了")` 原厂实际报时；各是独立单轮 PASS，不等于完整自动闭环。
- V64 已有 8 种真实原厂拒答文案严格白名单；识别 ASR 听不清与知识拒答的区别；日志中拒答和正常设备行为可通过 `dialog_id` 关联（离线成立）。

## 2. 真实 APK 固定依据

- LX04 `/system/app/MicoLauncher/MicoLauncher.apk`，57,794,254 bytes，5 DEX，SHA256 `803b6d89ef20df27f563e4cbec109ef7bec1d83c9bdc67990789fc72dd5be71a`。
- `InstructionCapabilityImpl.process(Instruction):boolean` 接受并登记云端指令，私有预处理含 Speak/Handler 分支，因此**禁止直接跳过 process 或随意返回 true/false**。
- `OperationManager.addInstructions(String,List):void` 是操作队列批量入口。可以尝试按同一会话过滤将执行的 Toast/Speak，但**上游 Handler/PCM 可能更早开始**。
- `TemplateToastOperation` 可能触发 `AsrTtsCard.onTtsRequest` 或 `hideTts()`；`SpeechSynthesizerSpeakOperation` 为独立语音操作；`SpeechSynthesizerCapabilityImpl.onPlayStart(int,String)` 含 dialog，而 `onPcmData(byte[])` 本身无 dialog 参数。
- 历史 `mico(2).log`：6 轮明确拒答，3/6 的 onPlayStart 早于 Toast，至少 1/6 的播放器事件早于 Toast；**批量操作过滤不能直接保证零首字发音**。
- 严禁使用不匹配的第三方 `OpenMico/MicoLauncher` 源码替代真实 APK。

## 3. V66 本轮实际代码变更

在独立分支 `test/lx04-v66-cloud-refusal-gate` 新增：

1. `experiments/lx04-v66-cloud-refusal/v66_cloud_refusal_gate.js`：真机 Java Hook 源码（两入口）：`InstructionCapabilityImpl.process` 只观察且原样调用一次；`OperationManager.addInstructions` 对同轮最终 ASR、`Nlp.StartAnswer/FinishAnswer`、Toast.query、Toast.text 和 Speak.text 严格关联。只按**完整已证实拒答原话**分类，冲突/未知/错轮原样放行。
2. `V66_EXPERIMENTAL_FILTER=false`（默认）：Hook 原厂调用仍原样执行，并仅记录匿名化确认状态；`true`（尚未获实机许可）：将本轮已确认拒答的 Toast/Speak 从**复制的操作列表**过滤后调用原厂一次。此为**实际编码的实验功能**，不是已验证安全的语音截断方案，因为 PCM/预处理可能早于批量分发。
3. `test_v66_cloud_refusal_gate.js`：Node VM 离线模拟，覆盖 8 条拒答 × 两种模式、正常 Toast × 两种模式、设备冲突、缺 final ASR、原列表不变和原厂方法恰好调用一次。
4. `README.md`：目的、运行、回滚、安全约束。

### V66 验收状态

| 阶段 | 目前状态 | 证据 |
| --- | --- | --- |
| V66 Java Hook 源码 | **已写入 GitHub 实验分支** | 见代码；未部署 |
| JS 语法检查/Node 模拟 | **已完成** | `node --check` 和 `V66_GATE_OFFLINE_PASS` |
| 实机 `process` 命中 | **未验证** | V61 失败时 `seen=0`，不能借用其 PASS |
| 实机 `addInstructions` 命中 | **未验证** | 本轮尚未注入 |
| 实机 Toast/Speak 无文字 | **未验证** | 批量过滤仅有离线模拟 |
| 实机零拒答首字、PCM 归属 | **未实现** | `onPcmData` 无 dialog 需另研究门控 |
| DeepSeek 接管 | **未接入该门控** | 旧已运行的 DeepSeek TTS/UI 保持 |
| GitHub 主分支和 LX04 正式 Agent | **均未变** | 此次只改隔离实验分支 |

## 4. 此前失败与约束，禁止重复踩坑

- V61：Logcat `unexpected EOF`，`seen=final=replay=0`，OEM PID 31483→13551，Frida owned cleanup `V57_OEM_PID_UNCLEAR_ABORT`；未获得有效 Hook 事件，不能声称失败或成功拦截。
- V62：只读显示 ADB 正常、OEM PID 13551 仅一例，但 Windows PowerShell `$Pid` 与系统只读 `$PID` 冲突，诊断未完成；必须改为 `$oemPid` 后再跑只读检查。
- Frida 17.25.1 在此 ARM32 曾出现 SIGBUS/ANR；现有短时试验以 Frida 16.6.6 为基线，仍需守住 PID 和 owner 安全前检。
- GitHub `test/lx04-routing-v15-ci` 用户明确说是**未更新的旧源码**。旧 `routeWindow=1800ms` 的无证据自动 DeepSeek、`acceptFallback` 等 TTS 播完、`onQuery/NativeRequest` 关键词二次路由与目标冲突；**待前置拦截实测成功，再整体改旧路由**。不能据它指责当前设备正在运行的版本。

## 5. 下一步实际实验顺序

1. **Windows 仅本地**：拉取 V66 分支至 `D:\Codex\XiaoSeek-apk`；`node .\experiments\lx04-v66-cloud-refusal\test_v66_cloud_refusal_gate.js`。本轮离线测试已通过。
2. **修复并完成 V62 只读 preflight**：精确检查 `pidof`/`ps -A`、Frida owned server、TracerPid、Logcat EOF；不自动 `kill`，遇到异常停下。
3. **V66-A 真机只读短时单轮**：`V66_EXPERIMENTAL_FILTER=false`，验证 `process` 和 `addInstructions` 进入、返回、匿名化 dialog 对应、正常报时不受影响；脚本中的 `V66_HOOK_ASSIGNED` 不是 Hook 命中证明。观察成功后复测一次已知拒答。
4. **V66-B**：独立研究 `onPlayStart`、`onPcmData`、Handler→`TtsPcmPlayer`，确认早到 PCM 所属 dialog 和可逆缓存/放行，再选择拦截点；**不能依赖仅 batch 过滤保证原厂语音不响**。
5. **V66-C**：只有声音与 UI 两路都能安全截断后，实机单轮确认原厂拒答音文均不出、Finish/唤醒恢复正常；随后增加 Agent `onConfirmedOEMRefusal` 专用入口，直接复用已有 DeepSeek Answer/TTS/UI，避免回到关键词路由，严格 once-only。
6. **真实回归**：报时、正常百科、音乐、米家设备、拒答、并发/断网/连唤/门铃优先，检查误拦与无人接管等故障。严格 fail-open OEM，不自动重试。

## 6. 文件与交接纪律

用户 Windows 持久路径 `D:\Codex\XiaoSeek-apk`；所有下载、编译缓存、日志、工具必须留 D 盘；本助手不能直接写用户本机 D 盘，生成的沙盒文件仅供用户下载后保存到 D 盘。每次有进度先更新此状态页，记录源码 commit/hash、验收、失败、下一步，保持此前 V65 完整历史，不重新讨论已通过能力。