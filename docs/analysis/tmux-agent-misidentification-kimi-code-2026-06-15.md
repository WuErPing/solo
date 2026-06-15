# Tmux Agent 检测误识别分析:`kimi --yolo` → `kimi-code`

> 本文档深度分析 `kimi --yolo` 被系统误识别为 `kimi-code` 并写入 `~/.solo/agent-commands.json` 的全链路根因。
>
> - **Status:** Analysis Complete(已修复)
> - **Date:** 2026-06-15
> - **Author:** Andy, Qoder
> - **Related:** [tmux-pane-analysis.md](tmux-pane-analysis.md), [tmux-pane-content-loading.md](../architecture/tmux-pane-content-loading.md), `daemon/internal/server/session_tmux.go`, `daemon/internal/server/agent_command_store.go`

---

## 1. 现象与证据

### 1.1 用户观察

`~/.solo/agent-commands.json` 里出现:

```json
{
  "agentName": "kimi",
  "launchCmd": "kimi-code",
  "lastSeen": "2026-06-14T19:34:48Z"
}
```

用户实际启动命令是 `kimi --yolo`,系统里没有 `kimi-code` 二进制(`which kimi-code` 返回 `not found`)。

### 1.2 现场三组证据

```bash
# 证据 1:ps 看到的是改写后的进程名
$ ps -p 90867 -o pid,ppid,args=
90867 90667 kimi-cod BUN_INSTALL=/Users/wuerping/.bun
#           ^^^^^^^^^ macOS 16 字节 comm 截断,实际是 kimi-code

# 证据 2:磁盘上的二进制叫 kimi,不叫 kimi-code
$ /Users/wuerping/.kimi-code/bin/kimi --version
0.14.3                  # 132MB Bun-compiled 二进制,入口叫 kimi

# 证据 3:tmux capture-pane -e 输出夹带 ANSI 转义码
$ tmux capture-pane -t %48 -p -e | od -c | head
\033[38;5;78m ... \033[1mkimi\033[0m --yolo
```

**结论:** kimi 进程在启动的瞬间,亲手抹掉了自己原本的命令行参数。

---

## 2. 检测链路七跳失败模型

把 `handleTmuxListAgents` 到 `~/.solo/agent-commands.json` 的完整链路拆开,每一跳都是一次信息丢失:

```
 ① tmux list-panes         →  pane_current_command = "kimi"      ✅ 还能看到真名
 ② parseTmuxPaneLines      →  agentName = "kimi"                  ✅ Layer1 命中
 ③ findAgentDescendant     →  遍历子进程树
 ④ ps -eo pid,ppid,args=   →  args = "kimi-code BUN_INSTALL=…"   ❌ 原始 argv 已被覆写
 ⑤ argsContainsAgentName   →  拿 "kimi-code" 去查 agentNames     ❌ "kimi-code" 不在表里
 ⑥ 回退:agentNames 全集匹配  →  "kimi-code" 在表里(新加入)       ❌ 把冒名者当正主
 ⑦ persist 到 JSON         →  launchCmd = "kimi-code"            ❌ 错误落地磁盘
```

**关键跳跃是 ③→④:** daemon 用 `ps` 作为**唯一的进程信息源**,而 `ps` 读的是内核 `user_addr_t` 指向的 argv 缓冲区——**这块缓冲区就是 `setproctitle()` 的写入目标**。信息源和污染源是同一个地址,所以从原理上就不可信。

---

## 3. 八大根因分解

### 3.1 根因 ① — kimi 二进制的 setproctitle 自改写(源头)

kimi 是 **Bun-compiled** 产物。Bun 运行时的启动流程有一段:

```
main()
  └─ bun_runtime_init()
       ├─ setproctitle("kimi-code")         // 用项目名覆写 argv[0]
       └─ argv_pushenv("BUN_INSTALL=…")     // 把安装路径塞进 argv 可见区
  └─ user_main("kimi", ["--yolo"])           // 用户代码根本拿不到原 argv
```

`setproctitle(3)` 的底层是**直接覆写进程的 argv+envp 连续内存区**。macOS 的 `ps` 命令通过 `sysctl(KERN_PROCARGS2, ...)` 读这块内存,所以**ps 永远只能看到改写后的版本**——`kimi --yolo` 在进程启动的几毫秒内就永久消失了。

> 对比:`claude` 是 Node.js 打包,Node 默认不改 argv,所以 `claude --dangerously-skip-permissions` 能被 ps 完整保留。

### 3.2 根因 ② — daemon 把 `ps` 当唯一真相源

`listProcessTree()` 在 `session_tmux.go:337` 用 `ps -eo pid,ppid,args=` 拿一棵"进程树快照"。这是整个检测链的**单点信息源**。当信息源本身被污染,后续所有推断都建在沙子上。

**替代信息源(本次没用上):**

| 候选 | 可行性 | 限制 |
|---|---|---|
| `task_for_pid` + `mach_vm_read` | 读原始 argv | 需 root / entitlement |
| `libproc.h` 的 `proc_pidinfo` | 系统 API | 同样读污染后数据 |
| `/proc/<pid>/cmdline` | 未污染 | 仅 Linux |
| `~/.zsh_history` / `~/.bash_history` | **唯一未被污染的原始记录** | 需关联 pane ↔ shell ↔ HISTFILE |
| tmux 输入事件挂钩 | 按键瞬间捕获 | 需改造 tmux 集成 |

### 3.3 根因 ③ — 双重匹配策略的"全集回退"引入了冒名顶替

`extractAgentLaunchCmd` 的匹配分两步:

```go
// 第一次:精确匹配已识别的 agentName="kimi"
if _, _, args := findAgentDescendant(panePID, {"kimi": true}); args != "" {
    return args
}
// 第二次:全集回退(为了处理 kimi→kimi-code 这种别名)
_, _, args := findAgentDescendant(panePID, agentNames)
return args
```

设计意图是"tmux 报 kimi,但二进制可能叫 kimi-code"。但当 `agentNames` 把 `kimi-code` 也加入(用户在 `config.go` 加的),**冒名者本身就是已知 agent**,回退策略变成了"把改写后的名字当成正主"。

`findAgentDescendant` 的判定只问"这个进程名在不在表里",不问"它是不是那个改名后的同名实体"——**没有"别名身份关联"的概念**。

### 3.4 根因 ④ — `isSuspiciousPsArgs` 初版启发式过弱

初版判定:

```go
return !namePresent && !hasFlags
// namePresent: 任一 token 的 basename 等于 agentName
// hasFlags:    任一 token 以 "-" 开头
```

`kimi-code` 场景:`namePresent=false`,`hasFlags=false` → 触发 fallback ✓
`kimi`(裸命令)场景:`namePresent=true`,`hasFlags=false` → **不触发** fallback ✗(漏报)

后来改成"first token basename ≠ agentName"才算可疑,加上"无 flag 也 fallback",才覆盖裸命令场景。

### 3.5 根因 ⑤ — ANSI 转义码让 fallback 的正则哑火

`tmux capture-pane -e` 输出(实测 `od -c`):

```
\033[38;5;78m user@host \033[0m  $  \033[1m kimi \033[0m  --yolo
```

正则 `kimi(\s+-\S+)+` 期望 `kimi` 后紧跟 `\s+-`,实际中间夹着 `\x1b[0m`。正则匹配失败 → `findLastAgentInvocation` 返回 `""` → fallback 链的最后兜底也没接住 → 只能把 ps 的脏数据交出去。

### 3.6 根因 ⑥ — capture 深度 `-50` 不够

`kimi --yolo` 被敲下 → kimi TUI 启动 → TUI 接管屏幕 → 命令行被推到 scrollback 深处。

- 初始 fallback 用 `-S -50`(50 行)
- 一个带 TUI 的应用启动日志通常 200+ 行
- 命令行**已经滚出 50 行窗口**

后改为 `-500` 才够。

### 3.7 根因 ⑦ — 持久化层没有"可疑数据"闸门

`AgentCommandStore.Merge` 只看 `LaunchCmd != ""`,不区分"这是用户敲的"还是"这是 ps 改的"。一旦脏数据写入 `~/.solo/agent-commands.json`,就成为**下一次扫描的"历史真相"**,自我强化。

### 3.8 根因 ⑧ — 加载期缺乏"二进制存在性"校验

旧 `load()` 直接 `json.Unmarshal` → `s.entries = entries`,**没有校验 LaunchCmd 的第一 token 是不是 PATH 上真实存在的二进制**。

`kimi-code` 在磁盘上不存在(`which kimi-code` 返回 `not found`),但 JSON 里就敢存、就敢用。这是最后一道本可以拦截却没拦的防线。

---

## 4. 八大根因的级联关系图

```
            ┌─────────────────────────────────────────┐
            │  根因①  kimi Bun runtime setproctitle   │  ← 污染源头
            └────────────────┬────────────────────────┘
                             │ 改写内核 argv 区
                             ▼
            ┌─────────────────────────────────────────┐
            │  根因②  daemon 把 ps 当唯一信息源       │  ← 单点故障
            └────────────────┬────────────────────────┘
                             │ 返回 "kimi-code BUN_INSTALL=…"
                             ▼
         ┌───────────────────────────────────────────────┐
         │  根因③  全集回退让冒名者(kimi-code)上位       │  ← 错误决策
         └────────────────────────┬──────────────────────┘
                                  │
          ┌───────────────────────┼───────────────────────┐
          ▼                       ▼                       ▼
┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐
│ 根因④ 启发式过弱  │   │ 根因⑤ ANSI 干扰   │   │ 根因⑥ -50 太浅   │
│ (未及时触发回退) │   │ (回退正则哑火)   │   │ (回退样本不够) │
└────────┬─────────┘   └────────┬─────────┘   └────────┬─────────┘
         │                      │                      │
         └──────────────────────┴──────────────────────┘
                                │ 脏 launchCmd 出炉
                                ▼
            ┌─────────────────────────────────────────┐
            │  根因⑦  Merge 无闸门直接落盘             │  ← 污染持久化
            └────────────────┬────────────────────────┘
                             │
                             ▼
            ┌─────────────────────────────────────────┐
            │  根因⑧  load 不验 PATH 存在性           │  ← 二次污染
            └─────────────────────────────────────────┘
```

这是一个**八级无互锁的级联失败**:任何一级如果有拦截,最终都不会出现 `kimi-code`。但八级全部穿透,错误就一路流到了用户看到的 JSON。

---

## 5. 防御纵深视角

把八级按**防御纵深**重新排列:

| 防御层 | 应做的事 | 实际表现 | 根因 |
|---|---|---|---|
| **信号采集层** | 采集未被污染的原始命令行 | 用 `ps` 采已被污染的 argv | ① ② |
| **信号识别层** | 区分"真主"和"改名实体" | 只问"在不在表里",不做身份关联 | ③ |
| **异常检测层** | 识别"可疑"信号并拒绝 | 启发式太宽松 | ④ |
| **信号修复层** | 用备用源恢复真实数据 | ANSI 污染正则 + 采样深度不够 | ⑤ ⑥ |
| **持久化层** | 拒绝脏数据入库 | 无闸门 | ⑦ |
| **加载清洗层** | 启动时清理历史脏数据 | 不校验二进制存在性 | ⑧ |

**六层防御全部失守**,这是典型的"瑞士奶酪模型"事故——每层都有洞,洞刚好连成一线。

---

## 6. 已实施的修复(2026-06-15)

针对八级失败的每一级都加了对应拦截:

| 层级 | 修复 | 位置 |
|---|---|---|
| 识别层(根治) | 从内建 agent 列表移除 `kimi-code`;新增 `agentNameByPrefix` 前缀匹配(`kimi-code` → `kimi`),彻底杜绝"冒名者进入已知表"问题 | `config.go`, `session_tmux.go` |
| 异常检测 | `isSuspiciousPsArgs`:首 token basename ≠ agentName 即可疑 | `session_tmux.go` |
| 信号修复 | pane 内容作为 fallback;`-500` 深度 | `extractAgentLaunchCmd` |
| 信号修复 | `stripANSI()` 剥离 CSI/OSC/charset 转义 | `session_tmux.go` |
| 异常检测 | "裸命令(无 flag)+ pane 找到带 flag 的版本" → 升级 | `extractAgentLaunchCmd` |
| 加载清洗 | `isStaleLaunchCmd` = 可疑 + `exec.LookPath` 失败 | `agent_command_store.go` |

**前缀匹配规则:** `findAgentDescendant` 在精确匹配和 args-token 匹配都失败后,尝试 `comm == "<agent>-*"` 前缀匹配。这保证了:
- `kimi-code` 进程 → 识别为 `kimi` agent(即使 `kimi-code` 不在 built-in 列表)
- `kimi-cli` 进程 → 精确匹配 `kimi-cli`(优先于 `kimi` 的前缀匹配,因为精确匹配先执行)

**测试结果:** 70+ 个新测试用例覆盖启发式、ANSI 剥离、pane fallback、前缀匹配、加载清洗。`go test -short -race ./internal/server/...` 全绿。

**验证:** 用用户的真实 JSON 数据跑端到端测试,`kimi-code` 条目被加载期自动清理,5 个合法条目保留。

---

## 7. 最深层的设计哲学问题

> **"进程命令行"这个概念,从来就不是可信数据。**

操作系统把 argv 放在进程**自己可写**的内存区。这是一个**设计上就允许被篡改**的字段,和 `User-Agent` HTTP 头是同一种性质——任何基于它做身份判定的代码,都预设了一个不该预设的信任。

真正可信的"用户敲了什么"只有三个地方:

1. **Shell 历史文件**(`~/.zsh_history`、`~/.bash_history`)— 由 shell 在命令执行前记录,kimi 启动后改不了。
2. **tmux 输入事件流**(如果能挂钩 tmux server)— 在按键被传给 pane 的瞬间捕获。
3. **pty 主端的写端日志**(daemon 自己 fork pty 时才能拿)— 当前架构里拿不到。

目前架构里,daemon 是个**旁观者**,不控制 pty,不挂钩 shell,只能事后用 `ps` 和 `capture-pane` 这两个"已被污染 / 部分污染"的接口反推。在这个约束下,**任何识别算法都不可能 100% 正确**——因为信息论上,你没法从被污染的信号里无损还原原始信号。

所以**真正的根治**是架构升级:让 daemon 成为 pty 的中介者(像 tmux 那样),或者挂钩 shell 的 history 写入。当前的 ANSI 剥离 + 启发式 + PATH 校验是**在现有架构下能做到的极限**,但永远会有漏网的边缘情况。

---

## 8. 一句话结论

> `kimi --yolo` 变成 `kimi-code` 不是一个 bug,是**八层防御被一个 Bun 运行时的 setproctitle 调用一次性击穿**的系统性失败,根因是 daemon 信任了设计上就不可信的 `ps` 信号,并在信号采集、识别、异常检测、修复、持久化、加载清洗六个层面都没有足够的拦截。

---

## 附录:受影响代码路径

| 文件 | 函数 | 角色 |
|---|---|---|
| `daemon/internal/server/session_tmux.go` | `handleTmuxListAgents` | 入口 |
| `daemon/internal/server/session_tmux.go` | `scanTmuxAgents` → `parseTmuxPaneLines` | 4 层 agent 识别 |
| `daemon/internal/server/session_tmux.go` | `extractAgentLaunchCmd` | 提取启动命令(已修复) |
| `daemon/internal/server/session_tmux.go` | `findAgentDescendant` / `listProcessTree` | ps 数据源 |
| `daemon/internal/server/session_tmux.go` | `findLastAgentInvocation` / `stripANSI` | pane fallback(新增) |
| `daemon/internal/server/session_tmux.go` | `isSuspiciousPsArgs` / `isStaleLaunchCmd` | 启发式判定(新增) |
| `daemon/internal/server/agent_command_store.go` | `load` / `Merge` | 持久化 + 加载清洗 |
| `daemon/internal/config/config.go` | `builtInTmuxAgentNames` | 已知 agent 表(`kimi-code` 已移除) |
