# LX04 V66 | 原厂云端拒答前置操作门控（实验源码）

## 唯一目标

小爱原厂唤醒/ASR/云端正常工作。只有原厂返回**同一 `dialog_id` 的已确认知识拒答**时，才在原厂 UI/TTS 执行前过滤拒答指令，后续再交给现成 DeepSeek。原厂报时、设备控制、音乐、正常回答、无关会话均不动。**不根据用户问题关键词分流**。

本目录是源码级真正修改，**不是已在设备上通过的成品**。无需修改原厂 APK 文件；通过原厂真实类进行 Frida 注入研究。

## 源码

- `v66_cloud_refusal_gate.js`：真实原厂 Java Hook：
  - `InstructionCapabilityImpl.process(Instruction):boolean`：原厂调用一次，收集同轮 final ASR、NLP 生命周期、Toast 和 Speak；
  - `OperationManager.addInstructions(String,List):void`：在队列加入前对完整指令列表按同轮语义判断；
  - 默认 `V66_EXPERIMENTAL_FILTER=false`：只读、原样放行；
  - **实验开关** `true`：候选明确拒答时，仅从复制列表中去掉 `Template.Toast` / `SpeechSynthesizer.Speak`，原厂 `addInstructions` 仍调用一次。此功能有真实实现，但还**不能保证 PCM 静音，禁止当成生产方案**。
  - 不修改 Recognize，不禁用 NLP/TTS，不重复 Nlp.Request，不调用 DeepSeek，不全局静音。日志不给出原始语音、完整 dialog ID。
- `test_v66_cloud_refusal_gate.js`：Node 内建 `vm/assert`，模拟两种开关、8 条拒答、3 条正常/听不清及冲突/缺 ASR，不需要音箱。

## 先执行离线测试

仅在 `D:\Codex\XiaoSeek-apk` 的仓库工作树：

```powershell
node .\experiments\lx04-v66-cloud-refusal\test_v66_cloud_refusal_gate.js
```

预期：`V66_GATE_OFFLINE_PASS`。

## 实机纪律

本次只提交研究源码，**没有对 LX04 设备注入**，也不修改正在运行的 Agent。

1. 必须先核对实际 APK SHA256 `803b6d89ef20df27f563e4cbec109ef7bec1d83c9bdc67990789fc72dd5be71a`。
2. 必须先完成 V61 Frida Server 清理状态、V62 `$Pid` 变量错误与旧 Logcat EOF 的只读检查；不得自动重试。
3. 在设备上首次使用只读 `V66_EXPERIMENTAL_FILTER=false`，先验证两个 Hook 的命中次数和会话归属；`V66_HOOK_ASSIGNED` 本身不是 PASS。
4. 已有真实日志证实 `onPlayStart` / PCM 可能在 Toast 前到达；此版本**不控制 PCM**。禁用实机 `true` 模式，直到 PCM 早到、Buffer/AudioTrack 归属和队列状态被确认。
5. 不用全局 `stopTts()`/master mute，不杀 OEM / Agent；Frida Server 仅按本轮独立 owner marker 安全清理。
6. 即使未来操作列表过滤成功，也还必须验证文字确实不显示、声音确实不输出、会话 Finish 正常、下一轮唤醒和正常技能不受影响，最后才接 DeepSeek。

## 为什么不直接修改旧路由

`test/lx04-routing-v15-ci` 是用户确认**故意没有跟随 V61–V65 更新的旧快照**；当前还缺最终拒答真正无声拦截的实机证据。V66 建独立实验分支，旧正式代码不动。待前置门控通过后，再把 **确认拒答事件** 单次交给已有 Agent 的 DeepSeek 入口，替换旧 1.8 秒/关键词/等待 TTS 播完路由。