# XiaoSeek LX04 V68 — 拒答事件诊断 Hook（只观察）

## 目的

V67 已经在 LX04 实机上命中 `InstructionCapabilityImpl.process` 16 次、`SpeechSynthesizerSpeakOperation.onProcess` 1 次，但 `candidate=0`、`TemplateToastOperation.onProcess=0`。V68 不改判定、不拦截、不接 DeepSeek；它只把 V67 未输出的事件信息补齐，定位 candidate=0 的具体原因：

- `dialog_id` 是否存在；
- 是否收到 `SpeechRecognizer.RecognizeResult` 最终 ASR；
- 是否收到 `Nlp.StartAnswer`；
- 是否收到 `Template.Toast`；
- Toast.query 是否与同轮 ASR 一致；
- Toast.text 是否命中 8 条完整拒答白名单；
- 是否收到同轮 `SpeechSynthesizer.Speak`，其文本是否与 Toast 一致；
- 是否出现原厂正常结果冲突。

V68 只输出事件名、会话别名、文本长度、布尔匹配与白名单编号，不输出原始用户问题、完整拒答文案或真实 dialog ID。原厂方法仍然原样执行一次。

## 先处理 V67 清理失败

V67 最后报：

`V67_OWNED_SERVER_CLEANUP_ERROR=COMMAND_EXIT_6: ... device_v67_cleanup.sh[71]: awk: not found`

因此 V67 自己启动的 Frida Server 很可能仍然存在。不要直接重跑 V67。V68 包中的新清理 helper 不依赖 awk，只依据 `owned-server.pid`、Frida Server 完整命令行、已知 SHA256 和当前 OEM PID/TracerPid，向该唯一 Server 发送 SIGTERM；不使用 SIGKILL，不碰 OEM/Agent。

## 执行

解压到 `D:\Codex\XiaoSeek-apk`，目录为 `v68-oem-refusal-trace`。

### 1. PowerShell 语法检查

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "D:\Codex\XiaoSeek-apk\v68-oem-refusal-trace\03_V68_Syntax_Check.ps1"
```

### 2. 清理 V67 遗留 Server

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "D:\Codex\XiaoSeek-apk\v68-oem-refusal-trace\00_V68_Cleanup_Existing_Server.ps1"
```

必须出现 `V68_CLEANUP_PASS=1`、`V68_OEM_PROCESS_UNCHANGED=1`，并且报告显示 Frida Server 进程数 0、marker ABSENT。若失败停止，不要手动 kill。

### 3. 运行 V68 实机只读观察

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "D:\Codex\XiaoSeek-apk\v68-oem-refusal-trace\02_V68_OEM_Cloud_Observe_Once.ps1"
```

出现 `V68_SAY_ONE_PREVIOUSLY_VERIFIED_HARMLESS_REFUSAL_QUERY_NOW=1` 后，对 LX04 说一次此前已验证的无害拒答问题（默认示例“介绍一下延迟器”）。不要在同一轮追加第二个问题。

### 4. 回传

只上传两个目录最新时间戳下的 `report.txt`：

- `D:\Codex\XiaoSeek-apk\v68-cleanup\...\report.txt`
- `D:\Codex\XiaoSeek-apk\v68-oem-refusal-trace\...\report.txt`

不要上传 `frida-*-PRIVATE.txt`。

## V68 验收边界

V68 通过只代表实机事件诊断完整、原厂进程稳定、Frida 清理成功；不代表拒答分类通过，更不代表任何输出已被拦截。只有事件轨迹确认同一 dialog 的 ASR/StartAnswer/Toast.query/拒答白名单/Speak/FinishAnswer 后，才设计下一阶段单轮文字拦截；PCM 必须另行验证，不能用全局静音替代。