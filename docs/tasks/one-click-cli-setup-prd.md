# PRD — 一键配置终端 AI 工具（One-Click CLI Setup）

> Status: spec · 2026-08-25 · author: Claude（待评审）
> **Owner: @sam**（Sam Wang）—— 拆出的 P1–P4 四张卡也都归他
> **看板短名（board prefix）: `One-Click CLI Setup`** — 本 PRD 拆出的每一张卡，
> `title` 都以它开头，逐字一致（`rules/adlc.md` §1）。不要写成 `One-Click Setup`
> 之类的近义词，那会在看板上静默裂成两组。
> Scope: 用户拿到 key 之后，把 **Claude Code / OpenCode / Codex CLI / Gemini CLI** 指向 DeepRouter 这一步的**自动化**。
> Parents（先读）: `CLAUDE.md` §0、`docs/onboarding-v2-prd.md` §7.5/§7.6、
> `docs/tasks/key-setup-guide-prd.md`、`docs/tasks/casual-journey-readiness-prd.md`。
> 代码: `web/default/src/features/keys/`（`lib/integration.ts`、密钥页），后端新增令牌与脚本下发端点。
> 素材: `deeprouter-ai/docs/integrations/{claude-code,opencode,codex,gemini-cli}.md`（各工具的权威配置格式）。

---

## 0. 这份 PRD 做什么（先读这个）

**一句话：让买了密钥（key）的用户不需要懂任何技术，就能把自己电脑上的 AI 工具接到
DeepRouter 上 —— 网页上勾一勾、贴一条命令，或者干脆只点一个按钮。**

用户买完 key 之后的最后一步，是把 key 填进自己用的 AI 工具里。`casual-journey-readiness-prd.md`
审计过八步旅程，结论是**前七步都已经绿了**：注册、充值、拿 key、自检、文案都做完了。
唯一没关上的就是这一步 —— 用户离开 DeepRouter 网站之后、自己配置工具的那一步。
它今天长这样（`docs/integrations/claude-code.md:39`）：

> Add these to your shell profile (`~/.zshrc`, `~/.bashrc`, or `~/.config/fish/config.fish`)

对一个刚付完钱的用户（律师、老师、创作者 —— 不是程序员），这句话里有四个他不需要知道的概念：
shell、profile、这三个文件分别是什么、以及改完要重载。

### 三个已核实的具体问题

| # | 问题 | 证据 |
|---|---|---|
| P1 | **Windows 没有任何路径** | `claude-code.md` 全文零次提到 Windows；方式 A 列的三个文件全是 Unix shell。23 份指南里只有 5 份提到 Windows，且都是 GUI 应用的下载链接 |
| P2 | **产品内片段在 Windows 上直接报错** | `lib/integration.ts:80` 生成 `# Run in your terminal` + `export ...`；PowerShell / cmd 不认识 `export`。`features/keys/` 全目录无任何平台检测 |
| P3 | **配完不知道成没成** | 用户只能自己开 `claude` 试。失败了不知道错在哪一步 |

### 方案一览：三条路，对应三类用户

| 用户用的是什么 | 我们给什么 | 规格在哪 |
|---|---|---|
| **终端工具**（Claude Code / OpenCode / Codex CLI / Gemini CLI） | 密钥页勾选工具 → 复制一条命令 → 贴进终端。脚本自动检测装了什么、挑一个真能用的模型、安全写入配置、实发请求验证、用人话汇报 —— 并且**一条命令能卸载干净** | **Track A**：§2 旅程 · §4.1–4.6 |
| **图形界面应用**（Cherry Studio 这类装上就点的聊天软件） | 密钥页一个按钮，**点一下应用自动配好**。用的是「深链」—— 一种点击后把地址和密钥直接塞给本机应用的特殊链接。机制已在生产上实测点通，只是入口藏着、指南一个字没提 | **Track B1 / B3**：§4.7 |
| **其他应用**（深链和脚本都覆盖不到的，如 Chatbox / Cursor） | 把接入文档改成 **AI 读得懂**的形态 —— 这类用户的真实做法是去问 AI，胜负在于 AI 能不能拿到正确资料，而不是靠猜 | **Track B2**：§4.7 |

### 现状（2026-08-27）

- ✅ 已拆成 4 张卡：**P1** 网页区块与一次性令牌 · **P2** 双平台脚本 · **P3** 深链入口与四份指南 ·
  **P4** 文档对 AI 可读。共 **43h、100 条验收**，明细见 §10，
  卡在 `deeprouter-ai/docs/adlc/tasks/one-click-*-task.md`。
- 🔴 动手实测（POC）推翻了第一版规格的相当一部分（详见文末附录 §0.1），并顺带发现**三个网关 bug**：
  ① `deeprouter-auto` 撞令牌白名单必 403 —— ✅ 已修复（ADR-0007），待部署；
  ② Gemini 协议转换错误（`top_k` 等）—— 待修，**挡着 P2 的 Gemini 那一份**；
  ③ Gemini / Responses 协议静默绕过智能路由 —— 待修。②③ 各有独立的修复卡。

### 怎么读这份文档

| 你想 | 去 |
|---|---|
| 看用户会看到什么 | **§2**（页面示意 + 终端输出示意） |
| 动手实现 | **§3**（范围与三条决策）→ **§4**（技术规格）→ **§5**（安全）→ **§6**（失败文案） |
| 验收 | **§7**（100 条，已按四张卡拆分） |
| 知道为什么不那样做 | **§8**（已关闭的问题）· **§9**（被否决的方案） |
| 查某条实测证据 | **文末附录 §0.1** —— 全文所有 **F / R / V / U 编号**（如 F1、R4）的出处 |

📌 正文凡与 §0.1 的实测冲突，**以 §0.1 为准** —— 第一版规格有 11 条是被它推翻后重写的。

---

## 1. 目标 / 非目标

**目标**

1. 用户在密钥页**复制一行、粘贴、回车**，四个终端工具中已安装的那些即被配好，全程不需要理解 shell / 环境变量 / Base URL。
2. 配完**当场证明能用**——脚本自己发真实请求并报出余额（`onboarding-v2-prd.md` §7.6 的「钱变成了算力」）。
3. Windows 与 macOS/Linux **都能走通**，且用户不需要知道自己该用哪条命令。
4. 需要写 shell 配置的工具，**用户的 shell 文件一辈子只被加一行**（§3 D2）。
5. **装得上，也卸得掉。** 一条命令把所有改动还原到跑脚本之前的状态（§4.6）。
6. **GUI 应用那半边人也不被丢下**（Track B，§4.7）：能用深链的（Cherry Studio / DeepChat / AionUI …）**点一下就配好** —— 机制已存在且 2026-08-26 已实测点通（§4.7 B1），只是埋在行菜单里没人找得到；**而指南里一个字都没提它存在**（B3）；深链覆盖不到的，至少保证用户问 AI 时 AI 能拿到**正确的官方资料**（B2）。

**非目标**

- ❌ 不做 Zed（key 在系统钥匙串里）与 OpenClaw（格式不稳定）——见 §3 D3。
- ❌ **不自动配置 GUI 应用**（Cherry Studio / Chatbox / Cursor …）——它们的设置在应用界面里，不是磁盘上的文件，脚本写不进去（§9）。**但不等于不管它们**，见 Track B。
- ❌ 不做桌面安装器（`.exe` / `.dmg`）。

> ⚠️ **本 PRD 有两条轨,拆卡时至少分成两张。** Track A（终端工具自动化）与 Track B（GUI 应用的资料可达性）目标一致、手段完全不同，一个是 shell 脚本 + 后端令牌，一个是文档与文案。放在同一份 PRD 里是因为它们回答的是同一个问题——**用户怎么把手上的工具接到 DeepRouter**；但它们不该是同一张卡。

---

## 2. 用户旅程（先看用户看到什么）

### 2.1 密钥页新增一块

```
你的调用密钥
sk-dr-••••••••••••••••XXXX                    [显示] [复制]

┌─ 一键配置 ────────────────────────────────────────────┐
│                                                       │
│  ① 选择要配置的工具                                    │
│                                                       │
│     ☑ Claude Code      Anthropic 官方 CLI             │
│     ☐ OpenCode         开源终端编码助手                │
│     ☑ Codex CLI        OpenAI 官方 CLI                │
│     ☐ Gemini CLI       Google 官方 CLI                │
│                                                       │
│     ⓘ 只会配置你勾选**且已安装**的。没装的会跳过并告诉你。│
│                                                       │
│  ② 打开终端，粘贴这一行，按回车                         │
│                                                       │
│    curl -fsSL https://deeprouter.co/i/A1B2C3 | sh     │
│                                            [复制]     │
│                                                       │
│  这条命令将配置：Claude Code、Codex CLI                │
│  15 分钟内有效  ·  想先看看这条命令做了什么？           │
└───────────────────────────────────────────────────────┘
```

**勾选状态编码进令牌**，命令本身不变形。改勾选 → 重新生成令牌 → 用户重新复制。

> ⚠️ **为什么选择放在网页上，而不是脚本里问？**
>
> §4.2.2 已经说明脚本无法交互（stdin 是那根管道）。而**参数传递在两个平台上极不对称**：Unix 的 `curl … | sh -s -- codex gemini` 很自然，但 PowerShell 的 `irm … | iex` 不接受参数，得写成 `& ([scriptblock]::Create((irm …))) -Only codex` —— 对目标用户等于劝退。
>
> 令牌携带勾选结果，两个平台的命令形状因此保持一致，而**网页恰好是唯一有真正 UI 的地方**：能列出工具全名、能加说明、能让人看清将要发生什么再决定。
>
> 命令下方那行「**这条命令将配置：…**」是必需的：`curl` 管道执行最大的顾虑就是「它到底会动什么」，把答案直接印在命令旁边。

> 📌 默认全部勾选，但**必须可见可改**。一键的价值在于「不想操心的人点一次就好」，而不在于「不给人选择」。

> 🔴 **勾选框里必须出现工具的品牌名**（Claude Code / OpenCode / Codex CLI / Gemini CLI）——
> 这是 `CLAUDE.md` §0 Rule 1「casual 界面不出现第三方客户端品牌名」的**有意例外**，
> @sam 2026-08-27 拍板。理由：这几个勾选框问的是「你装了下面哪几个」，
> **不点名就没法问** —— 「你的终端 AI 工具」这种说法，用户无法拿去和自己机器上装的东西对上。
> 那条规则防的是「本可以用大白话却偏用术语」，而这里**没有对应的大白话**。
> 同理，本区块**不做 persona 门控**：装了 Claude Code 的人就是它的用户，与控制台给他贴的
> persona 标签无关。

> 📌 **多把密钥时页面必须说清配的是哪一把。** 上面的示意图只画了一把，实际用户常有多把；
> 静默取「排序第一的那把」会把一把**带模型白名单**的密钥配进四个工具，而失败不会当场发生 ——
> 是几天后在工具里冒出一个 403，用户完全无从知道是选错了密钥。
> 做法：只有一把时不显示任何多余控件；多于一把时给选择器，**默认最新创建的那把**
> （刚建完就来配置是最常见的路径），并在选中受限密钥时当场提示。

Windows 用户看到的是 `irm https://deeprouter.co/i/A1B2C3 | iex`，**页面按 UA 自动切换**，用户不需要知道两者的区别。

⚠️ **Windows 必须写清楚开哪个终端**：

> 按 `Win + X` → 选「终端」或「Windows PowerShell」。
> **不要用「命令提示符」(cmd)** —— 那里两条命令都跑不了（cmd 里没有 `sh`，也没有 `irm`/`iex`）。

「命令提示符」恰恰是很多 Windows 用户唯一知道的那个。不写这句，他会打开 cmd、报错、然后认为 DeepRouter 坏了。

### 2.2 终端里自己跑完

> ⚠️ **下面是示意,不是逐字规格** —— **脚本的真实输出是英文**(§0.1 F21):用户群里有不读中文的人。
> 这段用中文写是为了让读 PRD 的人看懂流程与信息层次;**该出现哪些信息、分几步、说什么**才是规格,措辞不是。

```
DeepRouter 一键配置

  正在检测已装的工具...
  ✓ 找到 Codex CLI
  ✓ 找到 Gemini CLI
  ⏭ 找到 Claude Code，但它已用 Anthropic 订阅登录 —— 跳过
     换成 DeepRouter 会让你两头付钱。确实要换：加 --force claude-code
  · 没装 OpenCode，跳过

  正在写入配置...
  ✓ Codex CLI     (原文件已备份到 config.toml.bak-20260825-2143)
  ✓ 环境变量       (写入 ~/.deeprouter/env.sh)
  ✓ 已在 ~/.zshrc 加入一行引用

  正在验证...
  ✓ OpenAI 协议      通了   (Codex)
  ✓ Gemini 协议      通了   (Gemini CLI)
  · Anthropic 协议   未验证（Claude Code 已跳过）

  ────────────────────────────
  配置完成，当前余额 ¥87.42

  ⚠️ Codex 与 Gemini 需要新开一个终端窗口才生效
     （或者先运行： source ~/.zshrc）

  ⚠️ Gemini CLI 首次启动若引导你用 Google 登录，请选「API key」那一项

  想撤销？ curl -fsSL https://deeprouter.co/uninstall | sh
```

> 只验证**实际配了的**工具的协议。跳过的工具不发验证请求，也不报成失败——它没配，不是没配成。

**若 Claude Code 未被跳过**（用户没有订阅，或加了 `--force`），末尾还要给出首次启动的提示：

```
  Claude Code 现在直接运行 claude 就能用。
  ⚠️ 第一次运行 claude 会问 Is this a project you trust?，按回车确认即可
     —— 你走的是 DeepRouter，不需要 Anthropic 账号
```

> ⚠️ **这类首次启动步骤脚本做不到，但必须说。** 用户可以完全没有 Anthropic 账号——`docs/integrations/claude-code.md` 的 Prerequisites 只要求 DeepRouter 账号、DeepRouter key、装好 Claude Code 三样。不提示的话，用户会以为配置没生效——**这正是本卡要消灭的那种困惑，却发生在脚本管不到的地方。** 同类情形还有 Gemini CLI：它可能引导用户走 Google 登录，指南要求选 **API key** 那一项。凡是这类「脚本自动化不了的首次启动步骤」，都必须在输出里点名。
>
> 🔴 **具体提示什么，以当前版本实测为准，别照抄本 PRD**（§7 已把这条写进验收）。早先版本这里写的是「按 Esc 跳过 Anthropic 登录」—— F1 改走环境变量后**根本不会再弹登录**，F4 也实测 Esc 无效；当前版本（v2.1.246）实测到的是「Is this a project you trust?」信任确认。引导流程由工具方决定、随版本变。

> ⚠️ 「要不要重开终端」必须**说清楚且分工具说**：
>
> | 工具 | 读什么 | 要不要重开终端 |
> |---|---|---|
> | **OpenCode** | 配置文件 | ❌ 立即生效 |
> | **Claude Code** | 🔴 **环境变量**（F1） | ✅ 要 |
> | **Codex** | 环境变量（`env_key`） | ✅ 要 |
> | **Gemini** | 配置文件 **+** 环境变量（F10） | ✅ 要 |
>
> 🔴 **早先版本写的是「Claude Code / OpenCode 读配置文件，立即生效」—— F1 之后已不成立。** 四个工具里只剩 OpenCode 不需要重开。不写清楚，用户会以为没配上。

---

## 3. 范围与三条决策

四个工具的**真实**配置方式（来源：`docs/integrations/*.md`，逐份核对）：

| 工具 | 协议 | Base URL | 配置文件 | Key 落在哪 |
|---|---|---|---|---|
| **Claude Code** | Anthropic 原生 | `<base>`（**无** `/v1`） | — | 🔴 **环境变量**（实测 `settings.json` 的 env 块无效，§0.1 F1） |
| **OpenCode** | OpenAI 兼容 | `<base>/v1` | `opencode.json`（路径问工具，Q2） | ✅ 同文件 `options.apiKey` |
| **Codex CLI** | 🔴 **OpenAI Responses**（`wire_api = "responses"`，§0.1 F13/F14） | `<base>/v1` → 实际打 `/v1/responses` | `~/.codex/config.toml` **或**独立 profile 文件（Q7，✅ 已实测） | ⚠️ 环境变量（TOML 里只写变量名 `env_key`） |
| **Gemini CLI** | Google 原生 | 🔴 `<base>`（**不带** `/v1beta`，CLI 自己拼，§0.1 F11） | 🔴 `~/.gemini/settings.json` 必须写（F10） | ⚠️ 环境变量 |

🔴 **`<base>` 不是常量。** 存在多个独立部署，各自的 `server_address` 不同、数据库不同、**密钥不通用**（§0.1 F2）。脚本必须从令牌兑换接口取，**不得内置任何默认值**。

### 🔴 四个工具，四个不同的 Base URL，四种协议

这是本卡最容易出错的地方，也是唯一「配错了也不报错、只是安静地不工作」的地方：

- 给 Claude Code 写了带 `/v1` 的地址 → 它会拼成 `/v1/v1/messages`
- 给 Gemini 写了 `/v1` 而不是 `/v1beta` → 它说的是 Google 方言，走不通
- 三条都必须**各自独立验证**，不能发一个笼统请求就当全过了（见 §4.4）

### D1 · Phase 1 = 四个工具全做

Claude Code、OpenCode、Codex CLI、Gemini CLI。覆盖文档站「AI CODING ASSISTANTS (TERMINAL)」分类的全部四项。

> 🔴 **Gemini CLI 的前提**（§0.1 F17）：模型名可经 `settings.json` 的 `model.name` 设置（F12），
> 堵点在**网关的协议转换** —— 待卡「Fix: Gemini-protocol requests fail on every model…」的
> `top_k` 那半（约 2h）修好即可，工具侧无额外工作。

### D2 · 写 shell 配置：只加一行引用，绝不塞内容

> 🔴 **本条已按 POC 实测反转（§0.1 F1）。** 原规格把「写工具自己的配置文件」列为第 1 优先，Claude Code 走那条、不碰 shell。**实测不成立**：项目级 `.claude/settings.json` 的 `env` 块不能让 Claude Code 跳过登录，只有真环境变量可以。
>
> 我当初选 `settings.json` 的理由是「更干净：持久、跨平台、不碰用户的 shell」——理由本身没错，**只是那个方式不работа**。

现在的优先级：

1. **工具自己的配置文件** —— **只剩 OpenCode**（key 就写在 `opencode.json` 的 `options.apiKey` 里，与登录态无关）。
2. **工具自己的 env 文件** —— **无人适用**。查证结果（§8 Q5/Q6）：Gemini 的家目录级 `.env` 会被 `.git` 打断、且官方文档自相矛盾；Codex 的直写字段只见于二手资料且名字带 `experimental_`。两者都不赌。
3. **环境变量：我们自己的文件 + 一行引用** —— **Claude Code、Codex、Gemini CLI 三个都走这条**（四个工具里的三个）。

> 🔴 **但 Gemini 不是「只」走这条** —— 它同时还必须写 `~/.gemini/settings.json`（§0.1 F10）。所以四个工具里有 **三个要写文件**（OpenCode / Codex / Gemini）、**三个要写环境变量**（Claude Code / Codex / Gemini），两边都占的是 Codex 与 Gemini。

> 📌 这条反转让 `~/.deeprouter/env.sh` 从「Codex/Gemini 的兜底」升级成**主机制**。好消息是它本来就要建；坏消息是 fish 语法、幂等、卸载这些原本只影响两个工具的要求，现在影响三个 —— **§7 里那条 🔴 fish 测试的分量随之上升**。

```
~/.deeprouter/env.sh          ← 我们的文件，每次重跑整体重写
~/.zshrc  末尾追加一行：  . ~/.deeprouter/env.sh
```

**为什么不是直接往 `.zshrc` 里追加 export：**

| | 直接追加 export | 一行引用 |
|---|---|---|
| 重复运行 | 越堆越多 | **天然幂等**（重写我们自己的文件） |
| 改 key | 又要动他的文件 | 不用碰 |
| 卸载 | 要在他文件里找我们那几行 | 删一行 |
| 用户文件被改的次数 | 每次都改 | **一辈子一次** |

rustup 就是这个模式（`. "$HOME/.cargo/env"`）。

🔴 **但 Windows 不能这么镜像过去** —— 见下面的「Windows 是另一套机制」。

**🔴 shell 语法必须按 shell 类型发**：

| shell | 引用语法 | env.sh 里的赋值 |
|---|---|---|
| bash / zsh / sh | `. ~/.deeprouter/env.sh` | `export FOO=bar` |
| **fish** | `source ~/.deeprouter/env.fish` | `set -x FOO bar` |
| PowerShell | 🔴 **不写 `$PROFILE`** —— 见下 | 🔴 **没有 `env.ps1`** —— 见下 |

**fish 不认识 `export`。** 把 bash 语法写进 `config.fish`，用户每开一个新终端都会看到语法错误，必须有测试。

> ⚠️ 本 PRD 早先版本在这里写的是「这是本卡**唯一**真能让用户终端出问题的路径」——**那句是错的**。Windows 上有一个完全对应的，而且它命中的是**默认配置**，不像 fish 那样需要用户主动换过 shell。见下。

#### 🔴 Windows 是另一套机制，不是 POSIX 的镜像（2026-08-26 实测）

把「`$PROFILE` 里加一行引用 `env.ps1`」直接从 POSIX 抄过来，会撞上 **execution policy**：

```
$ powershell -ExecutionPolicy Restricted -Command ". profile.ps1"

. : File ...\profile.ps1 cannot be loaded because running scripts is
    disabled on this system.
DR_PROBE NOT SET
```

`Restricted` 是 **Windows 客户端的默认值**（`CurrentUser` 与 `LocalMachine` 都为 `Undefined` 时即生效）。后果有两层，第二层更糟：

1. 环境变量没设上，工具照旧要求登录；
2. **只要 `$PROFILE` 文件存在，用户此后每开一个 PowerShell 窗口都会看到一段红色报错** —— 而且报错指向的是我们写的文件。

**改用 `[Environment]::SetEnvironmentVariable(name, value, 'User')`**（写 `HKCU\Environment`）：

| | `$PROFILE` + `env.ps1` | `SetEnvironmentVariable(…,'User')` |
|---|---|---|
| 默认 `Restricted` 下 | ❌ 不加载 + 每次开终端报红 | ✅ **完全无关** —— 是 .NET 调用，不是脚本文件 |
| 覆盖范围 | 只有 PowerShell | 所有新进程，**含 GUI 应用与 cmd** |
| 幂等 | 靠整体重写我们的文件 | 天然幂等（同名覆盖） |
| 卸载 | 去 `$PROFILE` 里找那一行 | 置 `$null`，实测干净移除 |
| 用户文件被改的次数 | 一次 | **零** —— 不碰任何文件 |

> 📌 **`irm … | iex` 本身不受 execution policy 限制**（它不是脚本文件），所以会出现最坏的一种不对称：**安装脚本跑得好好的，配置却永远不生效**。用户看到「配置完成」，然后工具照旧要求登录。

**两种机制都要重开终端**，这点没有差别 —— 实测 `SetEnvironmentVariable` 写进注册表后，从当前进程再启动的子进程**读不到**（Windows 子进程继承的是父进程的环境块，不是注册表）。新开的终端才能拿到。§4.2 第 6 步的「分工具说明是否需要重开终端」在 Windows 上是**全部都要**。

> ⚠️ **「重开」必须是真的重开。** 从当前窗口里再启动一个终端**不算** —— 子进程继承的是父进程的旧环境块。POC #3 的说明里为此单列了一步「完全关掉这个窗口」，因为这是最容易把「机制不通」误判出来的地方。**用户引导文案里也要这么写**，不能只说「重开终端」。

**这条不是推理，是实测**（POC #3，§0.1 V5–V8）：写入 → 关窗 → 新窗读到全部四个变量 → 按名卸载后用户原有变量逐字还在 → 值里的 `%` 未被展开 → 全程**不需要管理员权限**。

> 早先版本的本 PRD 把「写 shell 配置」整体列为红线并排除 Codex/Gemini。那个判断**把风险讲重了**：追加一行语法正确的 export 不会弄坏 shell，nvm / rustup / pyenv / conda / homebrew 都在这么做。真正的风险窄得多，就是上面那张表里的 fish 语法问题和「文件结尾没换行」，两者都可预防。

### D3 · Zed 与 OpenClaw 仍然排除

- **Zed** —— key 存在**系统钥匙串**里，`docs/integrations/zed.md` 原文：*"Zed stores it securely in your system keychain — it does **not** go into `settings.json`."* 脚本写不进去。（指南提到可用 `DEEPROUTER_API_KEY` 环境变量替代，但同时未给出 `settings.json` 的路径，需另行查证，留待 Phase 2。）
- **OpenClaw** —— `docs/integrations/openclaw.md` 自带免责声明：*"OpenClaw is config-driven and **evolves quickly**... If your version's keys differ slightly, check OpenClaw's own docs."* 指南自己都不保证格式稳定，且未给配置文件路径。照着写等于埋一个随时会炸的维护点。

---

## 4. 技术规格

### 4.1 一次性令牌

- 密钥页在**已登录会话**下向后端申请，得到一个短令牌（如 `A1B2C3`）。
- 令牌：**单次使用**、**15 分钟过期**、绑定用户与该 key、**并携带用户在页面上勾选的工具清单**（§2.1）。
- 改动勾选 → 重新签发令牌 → 页面上的命令随之更新。**旧令牌不因此失效**（它自己 15 分钟后过期），但页面必须让人看清当前这条命令对应的是哪几个工具。
- `GET /i/{token}` 返回**脚本正文**（`text/plain`），密钥已由服务端嵌入。
- 🔴 **同时嵌入 Base URL** —— 取该实例自己的 `server_address`（`/api/status` 已在暴露）。**脚本里不得有任何硬编码的默认 Base URL**：存在多个独立部署，用户在哪个部署拿的令牌，配出来就必须指向哪个部署（§0.1 F2）。写错了的表现是**一路 401，且错误信息完全看不出是地址问题** —— 这一点在 POC 里真实消耗了半小时。
- 🔴 **密钥绝不出现在 URL 里。** URL 会留在 shell history、终端滚动区、录屏与截图中。令牌用完即废，泄露一个过期令牌没有价值。

### 4.2 脚本做的六件事

1. **检测** —— 三道闸，全过才配：
   1. **用户在页面上勾了它**（令牌携带，§4.1）—— 没勾的**碰都不碰**，也不打印任何东西
   2. **它真的装了** —— 🔴 **判据是可执行文件，不是配置目录**（§0.1 F9）：
      - 可执行文件在 PATH 上 → **装了**
      - 只有配置目录、没有可执行文件 → **不算装了**。告诉用户「找到了 X 的配置目录，但没找到 `x` 命令 —— 跳过」，**不要配置它**
      - 两者都没有 → 「你选了 X，但本机没找到」，不静默略过

      > ⚠️ **早先版本写的是「配置目录存在 *或* 可执行文件在 PATH」——那是错的。** 实测：装 **ChatGPT 桌面应用**会创建一个内容丰富的 `~/.codex/config.toml`（marketplaces / plugins / mcp_servers），但**不提供 `codex` 命令**。OR 判据下脚本会认定 Codex 已装、给它写配置、并告诉用户敲 `codex --profile deeprouter` —— 而那个命令不存在。ChatGPT 桌面版装机量很大，**这不是边缘情况**。
   3. **它没有生效中的付费登录**（§4.2.1）—— 有就跳过并说明

   > **勾选是意图，检测是现实，两者都要满足。** 页面不知道用户装了什么，脚本不知道用户想要什么 —— 各自只掌握一半信息，所以两道闸都不能省。
2. **检测 shell 类型** —— 决定 §3 D2 那张表里用哪套语法。
3. **合并写入** —— 见 4.3。
4. **写清单** —— `~/.deeprouter/installed.json`，见 4.6。卸载全靠它。
5. **验证** —— 见 4.4，**四种协议**各一次。
6. **人话汇报** —— 逐项 ✓ / 跳过，末尾报余额、**分工具说明是否需要重开终端**、并给出卸载命令。

### 4.2.1 🔴 已有付费登录的工具，默认跳过

**这是本卡唯一一个「脚本完全正常工作、却让用户损失金钱」的场景。** 其余所有失败态最坏是配置没生效并报错；这一条是**默默地开始花钱**。

> ✅ **检测信号已核实（2026-08-26）**：Claude Code 的登录态就是 `~/.claude/.credentials.json` 这个普通文件（实测存在，566b），**判断存在性即可**；Windows Credential Manager 里没有对应条目，不必碰系统钥匙串。
>
> 🔴 **存在即跳过，绝不读取内容。** 脚本不需要知道那是订阅还是 API key —— **任何**已有登录都不该被顶掉，所以「是哪种」这个信息没有用途。而一个手里已经握着用户密钥的脚本，再去解析用户的凭据文件，是白白扩大攻击面，也让「这脚本到底动了我什么」变得难以回答。§5 的安全要求同理。

一个用 Claude Max / Pro 订阅登录的用户，如果脚本往 `~/.claude/settings.json` 写进 `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN`，**他的订阅认证当场被顶掉**：Claude Code 转为按 token 从 DeepRouter 扣费，而每月已付的订阅在旁边闲置。**两头付钱，且无任何提示。**

而且这很常见——「想用 DeepRouter 跑 Codex」完全不意味着「也想让 Claude Code 换过去」。

**检测方式**（有则默认跳过，不静默覆盖）：

| 工具 | 已有付费登录的判据 |
|---|---|
| Claude Code | `~/.claude/.credentials.json` 中存在 `claudeAiOauth` |
| Codex CLI | `~/.codex/auth.json` 存在（官方 ChatGPT 登录） |
| Gemini CLI | Google 登录凭据存在 |

> ⚠️ 判据依工具版本而变，验收时以实测为准（同 §7 对首启引导的要求）。**判不准时的默认必须是「跳过并说明」，绝不是「覆盖」。**

### 4.2.2 为什么不能弹出询问

`curl … | sh` 下，**脚本的 stdin 是那根管道，不是终端** —— `read` 读到的是脚本自身的文本或直接 EOF，交互式提问跑不通。（可以靠 `/dev/tty` 绕开，但 Windows 的 `iex` 行为不同，两个平台会就此分叉。）

**因此默认行为本身必须是安全的，选择权通过参数给：**

```
--only codex,gemini       只配这几个
--force claude-code       我知道会顶掉订阅，仍然要配
--all                     等价于对所有已装工具加 --force
```

### 4.3 合并语义（硬约束）

- **读取原文件 → 只增改我们那几个键 → 写回。其余内容逐字保留。**
- **写之前先备份**为 `<原名>.bak-<时间戳>`，并在输出里告诉用户备份在哪。
- **原文件存在但解析失败 → 立即中止，不猜、不覆盖**，提示用户手工检查。
- **幂等**：重复运行结果一致，不产生重复键、不产生重复的引用行。
- ❌ **任何情况下都不得整体覆盖用户的配置文件。** 这是本 PRD 最重要的一条——冲掉别人的配置比不配置糟糕得多。

**各工具写入内容**：

#### 🔴 先定模型，再写任何东西

🔴 **元数据不可信，只有实发请求可信** —— 证据在 §0.1 F19（`/v1/models` 无价格字段、
两个模型接口互不包含、里面混着不能对话的模型、没有模型声明 `gemini`）。

**所以流程是「按优先级试，用真请求确认」，不是「查表算出一个」：**

1. 用 `deeprouter-auto` 发一次最小请求 → 通了就用它（能享受智能路由）
   🔴 **这一步不能省**：`deeprouter-auto` 曾因令牌白名单在生产 100% 失败（§0.1 R4，已修复待部署），
   而哪些部署带着修复、目录长什么样都随部署变 —— 探测是唯一不赌部署状态的做法。
2. 不通 → 取 `GET <base>/v1/models`（**令牌口径**，这是唯一能确定「这张令牌能用什么」的来源），
   按下面的顺序过滤与排序，得到候选列表：
   - 🔴 **先按 `supported_endpoint_types` 筛**：该工具用的协议必须在里面。
     Claude Code 要 `anthropic`；OpenCode / Codex 要 `openai`；
     **Gemini CLI 走的是转换路径 —— 要 `openai`，不是 `gemini`**（没有模型声明 `gemini`）。
   - 🔴 **剔除不能对话的模型**：名字含 `tts` / `audio` / `image` / `video` / `embed` / `rerank`，
     以及 `/api/pricing` 里 `quota_type == 1`（按次计价，13 个）的。
     ⚠️ 两条都要做 —— `quota_type` 只覆盖 `/api/pricing` 里的，那 4 个「能用但无价格」的查不到。
   - **再按便宜优先排序**：`/api/pricing` 有 `model_ratio` 的按它升序；**查不到价格的按名字启发式**
     （含 `mini` / `flash` / `haiku` / `nano` 的排前面），并把 `max_tokens` 压到最小。
3. 🔴 **逐个实发请求确认，第一个返回 200 的才写进配置。** 不要「算出一个就写」——
   元数据说得通的模型可能 404、可能是 TTS、可能预扣费超出余额。
   🔴 **探测请求的 `max_tokens` 必须与该工具实发的相当，不能用最小值**（§0.1 F20）:
   预扣费 = 单价 × `max_tokens`，实测同一模型同一账号 `max_tokens` 16 通过、4096 就 403，
   而 **Claude Code 实发要预扣 $0.16**。**用最小请求探测只能证明鉴权与模型可达，
   证明不了用户能真的对话** —— 会写下一份「验证通过、发一句话就 403」的配置。
   🔴 探测不到工具的真实值时，**把「这个模型每次对话约需预扣 $X」写进输出**。
4. 输出里说明用了哪个：`已配置模型：claude-sonnet-4-6（该部署未启用智能路由）`

🔴 **探测中的 403 要分三种，不能一律当余额不足：**

| 返回 | 含义 | 脚本该做什么 |
|---|---|---|
| `该令牌无权访问模型 X` | 令牌白名单挡的 | **换下一个候选**，不提示用户 |
| `预扣费额度失败，剩余 $A，需要 $B` | 余额不够跑**这个**模型 | **先换下一个更便宜的候选**；候选全试完才提示充值 |
| `404 does not exist` | `/v1/models` 列了但实际不可用 | **换下一个候选**，不提示用户 |

⚠️ **这三种今天都会发生在同一个部署上**，且都以 4xx 出现。把它们混成一句「配置失败」或
「余额不足」，用户拿到的诊断就是错的 —— 而 `gpt-4o-mini-tts` 那条尤其恶劣：
它会把用户送去**为一个永远不能对话的模型充值**。

> 不做这一步的后果：在只配了 Claude 渠道的部署上，`deeprouter-auto` 会退回硬编码的 `gpt-4o-mini`，而那个模型在该部署上没有渠道 → 用户拿到一份**必然报错**的配置。这条同时把原先只给 Gemini 开的例外**推广成通则**：任何工具都不写死模型名。

**Claude Code** — 🔴 **走环境变量，不写 `settings.json`**（§0.1 F1 实测）

```
ANTHROPIC_BASE_URL         = <base>          # 无 /v1
ANTHROPIC_AUTH_TOKEN       = <key>
ANTHROPIC_MODEL            = <model>
ANTHROPIC_SMALL_FAST_MODEL = <model>
```

写进 `~/.deeprouter/env.sh`（见 §3 D2 第 3 级），不碰 `~/.claude/settings.json`。

> ⚠️ 附带好处：**不碰用户的 `settings.json`，就不会碰他的 `permissions` / `hooks` / 主题等设置**，§4.3 那套 JSON 合并保护对 Claude Code 不再需要（只剩 OpenCode 要）。
> ⚠️ 附带代价：**要重开终端才生效**，Claude Code 因此从「立即生效」组挪到「需重开终端」组，§2.2 的输出文案要改。

**OpenCode** — 路径**先问工具**：`opencode debug paths` 解析出 config 路径，失败再回落 `~/.config/opencode/opencode.json`（见 Q2；它尊重 `XDG_CONFIG_HOME`，硬编码会对一部分用户失效。`%APPDATA%\opencode\` 是插件数据目录，**不是**主配置）

```json
{ "model": "deeprouter/<为 OpenCode 探测选中的模型>",
  "provider": { "deeprouter": {
    "npm": "@ai-sdk/openai-compatible",
    "name": "DeepRouter",
    "options": { "baseURL": "<base>/v1", "apiKey": "<key>" },
    "models": { "deeprouter-auto": { "name": "DeepRouter Auto" } }
}}}
```

> 🔴 **顶层 `model` 必写**（§0.1 F25）：只加 provider 不设默认，OpenCode 启动时仍用它自家目录挑的模型（实机：Nano Banana Pro，张口要 Google key），用户读作「没配上」。用户自设过的默认会被**有意**覆盖 —— 跑一键配置的本意就是把工具指向 DeepRouter（有备份、可卸载还原）。

**Codex CLI** — **分两种情况，两种都不需要合并 TOML**（见 Q7）· ✅ **机制已实测成立**（v0.149.1，2026-08-26）

内容都是同一段：

```toml
model = "<探测得到的具体模型>"   # 🔴 不是 deeprouter-auto，见下
model_provider = "deeprouter"

[model_providers.deeprouter]
name = "DeepRouter"
base_url = "<base>/v1"
env_key = "DEEPROUTER_API_KEY"
wire_api = "responses"        # 🔴 不是 "chat"
```

🔴 **`wire_api = "chat"` 在 v0.149.1 已被移除**，写了它 Codex **连配置都加载不了**：
`Error loading config.toml: 'wire_api = "chat"' is no longer supported.`
本 PRD 原文写的正是 `"chat"`（§0.1 F13）。

🔴 **改成 `responses` 之后，Codex 打的是 `POST <base>/v1/responses`，不是 `/v1/chat/completions`。**
已确认网关支持该端点（不带 key 探测返回 401 而非 404）。**§4.4 的验证表必须跟着改**（§0.1 F14）。

🔴 **Codex 也不能写 `deeprouter-auto`，原因与 Gemini 完全相同。**

改成 `wire_api = "responses"` 后，Codex 打的是 `/v1/responses`，而 **Responses 的请求体用 `input`、不用 `messages`**（`dto/openai_request.go:832`）。网关的 `chatRequestSnippet` 只认 `json:"messages"`，于是解析出零条消息 → `smart_router_no_messages` → **静默退回 `gpt-4o-mini`**。

> ⚠️ 本 PRD 早先版本写的是「**只有 Gemini 这一个工具不写 `deeprouter-auto`**」—— **那是因为当时只发现了 Gemini**。防住了发现了的那一个，没防住没发现的那一个。已归入 `deeprouter-ai/docs/adlc/tasks/gemini-smart-router-bypass-task.md`（因此卡 priority medium→high、估时 3h→5h）。

在那张卡修好之前，**Codex 与 Gemini 一样写探测得到的具体模型名**（§4.3 「先定模型，再写任何东西」），并在输出里告知“暂时拿不到智能路由”。

⚠️ 实测还带出一条印证性的 warning：`Model metadata for 'deeprouter-auto' not found. Defaulting to fallback metadata; this can degrade performance and cause issues.`

| 情况 | 写到哪 | 用户怎么用 |
|---|---|---|
| `~/.codex/config.toml` **不存在**（全新安装） | 直接写 `config.toml` | 直接 `codex` |
| `~/.codex/config.toml` **已存在** | 写 `~/.codex/deeprouter.config.toml`（**独立 profile 文件**） | `codex --profile deeprouter` |

**已存在时绝不改他的 `config.toml`。** 一个已经配过 Codex 的人对自己的默认模型有主张，劫持它本来就失礼；而这条同时把「合并 TOML」这个最大的工程不确定性一起消掉了。

> ✅ **这一条已实测坐实（本 PRD 最重要的一次正面验证）。** 造了一份「用户原有的」`config.toml`（`model = "gpt-5-codex"`），另写 `~/.codex/deeprouter.config.toml`，跑 `codex exec --profile deeprouter`：
>
> - 输出头部显示 `provider: deeprouter` / `model: deeprouter-auto` —— **独立 profile 文件确实被加载**
> - 请求打到 `https://deep-router.com/v1/responses`，返回 401（假密钥）—— **链路通**
>   （当时在**非正式部署**上测的；验的是 Codex 加载独立 profile 这个**工具侧机制**，与部署无关）
> - 那份「用户原有的」`config.toml` **逐字未变**
>
> 📌 **意义**：Q7 消除的是本卡原本最大的工程风险（在 sh 与 PowerShell 里手写 TOML 的增删改）。它此前只有文档依据 —— 而同样是文档依据的 Gemini（Q5）三条全错。**现在它有实证了。**

输出里必须**分情况告诉用户该敲哪条命令**，否则第二种情况的用户会以为没配上。

**Gemini CLI** — 🔴 **环境变量 + 一个 JSON 文件，两样都要**（原文写「走 D2 第 3 级（纯环境变量）」，**实测不成立**，见 §0.1 F10–F12）

以下全部经 v0.57.0 实测（2026-08-26）：

**① `~/.gemini/settings.json` —— 不写它，无人值守模式直接报 `Invalid auth method selected.`，一个请求都不会发出**

```json
{
  "security": { "auth": { "selectedType": "gemini-api-key" } },
  "model":    { "name": "<探测得到的具体模型>" }
}
```

- **键是嵌套的**：`security.auth.selectedType`。顶层的 `selectedAuthType`（旧版结构）**在 v0.57 上无效**，实测照样报错。
- 合法值来自 `AuthType` 枚举：`gemini-api-key` / `oauth-personal` / `vertex-ai` / `cloud-shell`。
- ⚠️ 存在环境变量 `GEMINI_DEFAULT_AUTH_TYPE`，但它**只在交互路径生效**（读它的代码在 `interactiveCli-*.js`）。设了它跑 `gemini -p` 照样失败。

> 📌 **这条把 Gemini 从「只写环境变量」推到「和 Claude Code 之外唯一要写 JSON 的工具」**，因此 §4.3 的合并保护、备份、卸载还原**全部适用于它**——这部分工程量原本没算。

**② 环境变量**

```
GEMINI_API_KEY=<key>
GOOGLE_GEMINI_BASE_URL=<base>          ← 🔴 绝不能带 /v1beta
```

- 🔴 **`GOOGLE_GEMINI_BASE_URL` 不能带 `/v1beta`** —— CLI 自己会拼。带了就变成
  `POST /v1beta/v1beta/models/…`，网关返回 `Invalid URL`。**与 Cherry Studio 的 `/v1/v1` 是同一个坑，方向相反**：Cherry 要你别写 `/v1`，Gemini 也要你别写 `/v1beta`。本 PRD 原文写的是 `<base>/v1beta`，**是错的**。

**③ 模型名 —— 写 `settings.json` 的 `model.name`，不是环境变量**

🔴 **`GEMINI_MODEL` 环境变量无效，而且原因不是「读了没生效」——它根本没被读。**
翻 v0.57 的 bundle：`env.GEMINI_MODEL` **零命中**；`GEMINI_MODEL` 在包里只是常量名
（`DEFAULT_GEMINI_MODEL = "gemini-3-pro-preview"` 之类），不是环境变量入口。

真正的键是 **`settings.json` 的 `model.name`**（消费点 `settings.model?.name`），**已实测生效**
——配 `gpt-4o-mini` 后请求确实带着 `gpt-4o-mini` 打到了网关。

- ✅ **零额外成本**：这就是 ① 那个文件，多一个嵌套键而已。合并保护 / 备份 / 卸载还原全都复用。
- 同一层还有 `model.maxSessionTurns` / `compressionThreshold` 等，**本 PRD 一律不碰**。
- 命令行 `-m <model>` 也有效，但脚本不能替用户往命令里加标志 —— 不用它。

> 📌 原文写的是「模型名的落点是 Gemini 上唯一没有结论的一项，原因未查清」。
> **那句「未查清」后来被当成结论用，据此把 Gemini CLI 判出了 Phase 1 —— 是错的**（§0.1 F17）。

**④ 🔴 无人值守有个信任门（F18）—— 不影响脚本，影响测它的人**

`gemini -p` 在未信任目录下**一个请求都不发**：

```
Gemini CLI is not running in a trusted directory.
```

- **脚本本身不受影响** —— 它只写配置、不代替用户运行 `gemini`，§4.4 的验证走的是脚本自己发的 HTTP 请求。
- 但凡是**人**拿 `gemini -p` 做测试或验收，要带 `--skip-trust` 或设 `GEMINI_CLI_TRUST_WORKSPACE=true`。
  ⚠️ 这个报错**和配置完全无关**，不知道的话会一路去查 key 和 base URL。

**⑤ 🔴 目前网关侧还修不通（F17）**

以上四条都配对了，Gemini CLI 仍然跑不通，**堵点在网关的协议转换**：
配 `gpt-4o-mini` → 400 `top_k`；配 `claude-sonnet-4-6` → 500 `not implemented`。
→ `deeprouter-ai/docs/adlc/tasks/gemini-protocol-conversion-task.md`。
**本 PRD 的 Gemini 那一份要等 `top_k` 那半修好才能验收。**

⚠️ **Gemini 与 Codex 两个工具都不能写 `deeprouter-auto`。** 同一个原因：它们的请求体都不用 `messages`（Gemini 用 `contents`，Responses 用 `input`），smart-router 的解析器取不到消息，`deeprouter-auto` 会静默退化成 `gpt-4o-mini`。具体模型名走 §4.3 的探测流程。

**共享环境变量文件** — `~/.deeprouter/env.sh`（fish 为 `env.fish`）

同时承载 `DEEPROUTER_API_KEY`（Codex 用）与上面三个 Gemini 变量。整体重写，不追加。

🔴 **Windows 上没有这个文件** —— 同样这几个变量改走 `[Environment]::SetEnvironmentVariable(…, 'User')`（§3 D2）。所以两个平台在这里**行为一致但机制不同**：变量名、值、生效范围都相同，落地方式一个是文件、一个是注册表。§4.6 的卸载因此也要分平台。

### 4.4 验证：四种协议各验一次

**不是发一个笼统请求就当全过了。** 按每个已配工具实际会走的协议分别验证：

| 协议 | 端点 | 覆盖 |
|---|---|---|
| Anthropic 原生 | `POST <base>/v1/messages` | Claude Code |
| OpenAI 兼容 | `POST <base>/v1/chat/completions` | **OpenCode**（已实测） |
| 🔴 **OpenAI Responses** | **`POST <base>/v1/responses`** | **Codex**（§0.1 F14，已实测） |
| Google 原生 | `POST <base>/v1beta/...` | Gemini CLI |

只有这样才能真正证明「这个工具配完能用」，而不只是「这把 key 有效」——四个 Base URL 各不相同，写错任何一个都不会在写入阶段报错。

同时满足两条既有硬约束：`key-setup-guide-prd.md` §6.1（展示值必须经活网关校验）与 `onboarding-v2-prd.md` §7.6（当场确认钱变成了算力）。

### 4.5 两个平台

| | 命令 | 脚本 |
|---|---|---|
| macOS / Linux | `curl -fsSL https://deeprouter.co/i/{token} \| sh` | POSIX sh |
| Windows | `irm https://deeprouter.co/i/{token} \| iex` | PowerShell 5.1+ |

两份脚本**行为必须一致**（同样的检测、备份、合并、验证、输出）。Windows 上的路径按 `%USERPROFILE%` 解析。

### 4.6 卸载

```bash
curl -fsSL https://deeprouter.co/uninstall | sh      # macOS / Linux
irm https://deeprouter.co/uninstall | iex             # Windows
```

**不需要令牌** —— 卸载不需要密钥，所以是一个静态地址，任何时候都能跑。

🔴 **但它同样要按 User-Agent 分平台下发**（P1 已建好这套机制，直接复用）。理由与安装那条完全相同：把 POSIX 脚本喂进 `iex` 不是「什么都没发生」，PowerShell 会把每一行读不懂的原样回显到终端。卸载脚本里没有密钥，所以泄露风险不存在，但用户会看到满屏红字然后以为卸载失败 —— 而这恰恰是他最不需要再受一次惊吓的时刻。

**为什么这是 Phase 1 而不是 Phase 2**：本 PRD 存在的全部理由是「用户干不了编辑 shell 配置文件这件事」。如果卸载说明写成「请打开 `~/.zshrc` 删掉那一行」，就等于让他去做我们花二十小时帮他避开的那件事——自相矛盾。而且**装了撤不掉**会直接放大 §5 对 `curl` 管道执行的信任顾虑。rustup / nvm / homebrew 都提供卸载，这是这类脚本的基本礼貌。

#### 安装时写一份清单（卸载的前提）

`~/.deeprouter/installed.json`，记录本次动过什么：

```json
{ "installed_at": "2026-08-25T21:43:00Z",
  "tools": [
    { "name": "claude-code", "file": "~/.claude/settings.json",
      "pre_existing": true,  "original_backup": "settings.json.bak-20260825-2143" },
    { "name": "codex", "file": "~/.codex/config.toml",
      "pre_existing": false, "original_backup": null }
  ],
  "shell": { "file": "~/.zshrc", "line": ". ~/.deeprouter/env.sh" } }
```

🔴 **没有清单，卸载就是猜。** 用户跑过两次安装就会有两个 `.bak-` 文件，其中**只有最早那个**才是他真正的原始状态；而脚本新建的配置文件根本没有备份可恢复，正确动作是删掉它而不是还原。清单里的 `pre_existing` 与 `original_backup` 就是用来区分这两种情况的。

**因此清单只在第一次安装时写入 `original_backup`，重复安装不得覆盖它。**

#### 卸载做的四件事

1. 读清单。清单不存在 → 明确告知「没有检测到本机的 DeepRouter 配置」，**不做任何猜测性删除**。
2. 逐个工具：`pre_existing: true` → 用 `original_backup` 还原；`pre_existing: false` → 删掉该文件。
3. **清掉环境变量** —— 分平台：
   - macOS / Linux：从 shell 配置里删掉那一行引用（**只删这一行**，其余逐字保留）。
   - 🔴 Windows：对清单里记下的每个变量名调 `SetEnvironmentVariable(name, $null, 'User')`。**按名字删，不是清空整个 `HKCU\Environment`** —— 那里面有用户自己的东西。实测置 `$null` 后注册表项干净消失。
4. 删掉 `~/.deeprouter/` 整个目录，然后汇报每一项。

> Windows 这条正是「安装不写文件」的代价：没有文件可删，所以**清单必须记下我们设过哪些变量名**，否则卸载无从下手。见上一小节的清单格式。

### 4.7 Track B — GUI 应用

GUI 应用（Cherry Studio / Chatbox / LobeChat / Cursor …）**正是 `onboarding-v2-prd.md` §3 写的目标人群**所用的东西 —— 律师、老师、创作者没有 Claude Code，他们有的是 Cherry Studio。Track A 一个都覆盖不到。

Track B 分**三级**，**先用现成的机制，再补文档**：

| | 做什么 | 修的是 |
|---|---|---|
| **B1** | 把深链入口从行菜单提到密钥页主视图 | **密钥页的发现性** |
| **B3** | 四份指南补上一键路径 | **指南在教难的那条路，且从不提简单那条存在** |
| **B2** | `llms.txt` / 事实块 / 求助文本 | **深链覆盖不到时，AI 能不能拿到正确资料** |

> 📌 **B1 与 B3 不互相替代。** B1 让入口在密钥页看得见，但**用户被指过去的地方是指南** —— 而指南今天从头到尾在教手动配置。

---

#### 🔴 B1（优先）· 深链一键配置 —— **机制已存在且已在生产运行**

`setting/chat.go` 定义了一组「聊天预设」，`web/default/src/features/chat/lib/chat-links.ts` 把密钥与 Base URL 打包进深链：

```ts
if (url.includes('{cherryConfig}')) {
  const payload = { id: 'new-api', baseUrl: safeServerAddress, apiKey: safeApiKey }
  const encoded = encodeURIComponent(toBase64(JSON.stringify(payload)))
  return replaceToken(url, '{cherryConfig}', encoded)
}
```

**用户点一下 → 操作系统把 `cherrystudio://` 交给 Cherry Studio → 它自动建好 provider，Base URL 和密钥都已填好。** 不需要脚本、不需要写配置文件、不需要用户找设置界面。

`GET /api/status` 已经在返回这份预设列表（正式部署 `deeprouter.co` 与另一套 `deep-router.com` 上均已确认）。

##### ✅ 已实测点通（2026-08-26，Cherry Studio / Windows）

**这原本是本 PRD 证据最弱的一处**（「代码在」≠「点下去真能配好」，与 F1 / F8 同一形状），
所以动工前真点了一次。全程 `⋯ → 聊天 → Cherry Studio → 允许 → 添加 → 获取模型列表 → 发消息`，
**用户不输入任何东西**，请求到达网关（403 预扣费，挡住的是余额不是机制）。

🔴 **三条结论，两条推翻了本 PRD 自己的预判：**

1. **尾斜杠坑没触发** —— 深链导入走的就是手输那条归一化路径，Cherry Studio 自动补 `/v1`。
   §4.7 B2 记的那条陷阱**只影响手输用户**。
2. **模型不用用户自己找** —— 早先预判「payload 无 models 字段，用户还得去抄模型 ID」是**错的**，
   Cherry Studio 自己会拉。那一步恰是小白最卡的地方，而它不存在。
3. ⚠️ **品牌显示为「New API」不是 DeepRouter**（服务商 ID `new-api`，映射到 Cherry Studio 内置预设）。
   **上游预设命名，改不了**，只能在引导文案里提前说明。

> 📌 另一处对照值得记进 §1 的取舍：同一个 403，**Claude Code 显示成 `Please run /login`（错的，F5），
> Cherry Studio 原样透传网关原文带确切数字**。**GUI 那条路上用户看到的错误更可能有用** ——
> 但那是运气不是契约，§6「脚本必须自己发验证请求并翻译错误」因此更站得住。

> ⚠️ **另立卡的文案问题（不属本 PRD）**：一句 `hi` 预扣 **$0.49**，那是按 `max_tokens` 预授权、
> 不是实际消耗。`key-setup-guide-prd.md` §5 只覆盖了**自检工具**里的文案；
> 用户在**第三方应用**里看到的是网关原文，没人管。

##### 深链覆盖面

| 覆盖方式 | 应用 |
|---|---|
| **深链，配置全填** | **Cherry Studio · AionUI · DeepChat** |
| URL 参数式 | LobeChat · AI as Workspace |
| 专用通道 | 流畅阅读（浏览器扩展直投）· CC Switch |
| ❌ 不在其中 | **Chatbox · Cursor · GitHub Copilot · Zed** → 只能靠 B2 |

**所以 B1 不是「开发一个功能」，是「把已有的功能露出来」。**

##### 真正的问题是发现性，不是能力

入口埋在密钥页**每行末尾的 `⋯` 行菜单**里（`features/keys/components/data-table-row-actions.tsx:288-298`）。

刚付完钱、正盯着「你的调用密钥」发愁的小白**不会想到去点那三个点**。他会照着下面那句
「粘到你正在用的 AI 工具的设置里」去 Cherry Studio 翻设置 —— 而**一个能替他全填好的按钮
就在同一个页面上，隔着一次他不会做的点击**。

✅ 已核实**没有** persona / casual 门控，「去品牌」那次改动没把它藏起来，它只是位置不对。

##### B1 要做的

- 把深链入口从行菜单**提到密钥页的主视图**，与「一键配置（终端）」并列成两个入口：**用终端工具 → 复制命令；用桌面应用 → 点一下**。
- 只展示**当前部署实际启用**的预设（读 `/api/status`，别硬编码）。
- 点击前说清会发生什么：「这会打开 Cherry Studio 并自动填好配置」。
- 深链失败（应用没装 / 协议未注册）要有可见的兜底 → 落到 B2 的指南。

> 工程量小得不成比例：机制、密钥解析、URL 构造全都写好了，改的是**入口的位置和措辞**。

---

#### 🔴 B3 · 指南在教手动路径，从不提一键存在

**B1 修的是密钥页的发现性，但指南才是用户真正被指过去的地方 —— 而它从头到尾在教难的那条路。**

`/resources/cherry-studio` 现在的「操作步骤」是：打开设置 → 添加服务商 → 起名字 → 服务商类型选 OpenAI → 粘 API key → 粘 API host → 滚到 Models → **输入一个来自 Model Catalog 的模型 ID** → 打开开关 → 检测。**十步，其中两步要用户自己产出他不可能知道的值**（Base URL、模型 ID）。

而实测的一键路径是：`⋯ → 聊天 → Cherry Studio → 允许 → 添加 → 获取模型列表 → 添加全部模型`——**零输入**。

**指南里一个字都没提它存在。**

##### 范围：只有 4 份指南对应的应用真有深链

| 应用 | 预设类型 | 指南 |
|---|---|---|
| **Cherry Studio** | 深链，配置全填 | `cherry-studio.md` |
| **CC Switch** | 专用通道 | `cc-switch.md` |
| **Lobe Chat** | URL 参数 | `lobehub.md` |
| **OpenCat** | 专用通道 | `opencat.md` |

→ **4 份 × 中英两份 = 8 个文件**。每份在「操作步骤」**之前**插入一段「一键配置（推荐）」，手动步骤降级为「或者手动配置」。

⚠️ **OpenCat 是 macOS / iOS 应用** —— 验证它需要 Mac，与 §0.1 U7 是同一个依赖。

##### 附带发现：5 个应用有一键能力，却没有任何文档

`AionUI` · `DeepChat` · `AI as Workspace` · `AMA 问天` · `流畅阅读（FluentRead）`

其中 **AionUI 与 DeepChat 是 `DEEPLINK-FULL`，与 Cherry Studio 同级别** —— 点一下就配好，却一个字的指南都没有。

> 📌 **这不属于 B3，是一个独立且更大的缺口**：产品已经具备的能力，用户无从知道。建议另立卡。（注意 `流畅阅读` 是 FluentRead，与已有的 `immersive-translate.md`（沉浸式翻译）**是两个不同产品**，不要混。）

##### 与域名卡的重叠

✅ 这 4 份指南（连同其余 19 份）硬写的 `https://deeprouter.co` 与 `https://api.deeprouter.co` **是对的** —— 本 PRD 早先版本说它们“全是错的”，那是基于已被订正的域名结论（§0.1）。因此那张域名卡已删除（结论已写进 `deeprouter-ai/deploy/README.md`）， **从「挡着本卡」降为「确认一遍」**，不再是硬依赖。

**B3 与它必须一起做**，否则同一批文件要被改两遍。B3 的卡应 `depends_on` 那张。

---

#### B2（兜底）· 让 AI 拿到正确资料

给深链覆盖不到的应用（Chatbox / Cursor / Copilot / Zed …）。

真实行为是：他们不会打开文档站，会直接问手边的 AI——

> 「我有一个 DeepRouter 的 key，怎么在 Cherry Studio 里用？」

**然后 AI 开始猜。而对 DeepRouter，猜出来的答案几乎必错**，因为这个产品的陷阱密度异常高：

| 陷阱 | 猜错的后果 |
|---|---|
| Claude Code 要**不带** `/v1` | 拼成 `/v1/v1/messages` |
| OpenCode / Codex 要**带** `/v1` | 404 |
| Gemini 要 `/v1beta` | 协议不匹配 |
| **Cherry Studio：带不带尾斜杠是两种语义** | `…/v1` 无斜杠 → `/v1/v1` → 404，且报错看不出原因 |

一个 AI 凭常识猜，大概率给出 `https://api.deeprouter.co/v1` —— **这个值对 Cherry Studio 恰好是错的。**

> Cherry Studio 那条不是我们推断的，是 `docs/integrations/cherry-studio.md` 里**唯一被单独立成一节**的适配问题（"One thing to know about the API host slash"）。最自然的写法恰好是错的。

#### 交付物 1 · 文档站对 AI 可读

23 篇指南**本来就是 markdown、本来就在线上**（运行时 fetch `public/docs/integrations/*.md`）。差的只是一个让 AI 找得到、读得懂的入口：

- **`/llms.txt`** —— 列出每个工具的指南地址与一句话说明
- **`.md` 原文可直接访问**，不必解析 SPA
- **每篇开头一个机器友好的事实块** —— 精确的 Base URL（含尾斜杠规则）、model、认证头、协议方言

事实块是关键：AI 抓到整篇散文仍可能提炼错，给它一段结构化事实就不会。

#### 交付物 2 · 密钥页生成一段可复制的「求助文本」

```
┌────────────────────────────────────────────────────┐
│  用 Cherry Studio / Chatbox 这类应用？               │
│                                                     │
│  复制下面这段，发给任意 AI，它会一步步教你：           │
│                                                     │
│   我要在 Cherry Studio 里用 DeepRouter。            │
│   官方指南：https://deeprouter.co/resources/…      │
│   Base URL：https://api.deeprouter.co             │
│   （注意：这个地址不要在末尾加 /v1）                  │
│   请照着指南告诉我每一步在哪里填。      [复制]        │
└────────────────────────────────────────────────────┘
```

🔴 **这段文本里绝不带密钥。** 用户会把它贴进第三方 AI，密钥一旦进去就等于交给了那家服务商。文本只给公开信息 + 指南链接，密钥由用户自己在应用里填。

#### ✅ 前提已定：真相源是 in-app 那份

指南有两份拷贝，长期没有声明过哪份是真的：

| 位置 | 内容 | |
|---|---|---|
| `deeprouter-ai/docs/integrations/` | 25 个文件，**纯英文** | ❌ 不是真相源 |
| `deeprouter/web/default/public/docs/integrations/` | **49 个文件**，含 `.zh.md` | ✅ **真相源** |

**判据是中文版**：站上「资源」区是双语的（右上角 English / 中文 切换），而只有 in-app 那份有 `.zh.md`。纯英文的那份不可能是真相源——中文读者看到的东西根本不在里面。详见 Q8。

⚠️ **把事实块加到 meta-repo 那份，网站上永远看不到，而且不会报错。** 又是一次静默失效——所以这条要在动笔前先落实，包括给旧那份加横幅或删除（`rules/docs-currency.md`）。

#### ❌ 不做：让 AI 直接替用户配置

见 §9。核心障碍是循环依赖，不是工程量。

---

## 5. 安全要求（硬约束）

1. **仅 HTTPS + 自有域名。**
2. **密钥不进 URL**（§4.1）。
3. **提供「先看看这个脚本」链接** —— 指向 `deeprouter/internal/connect/` 里的模板源文件（Q4；该仓库公开）。`curl` 管道执行是有争议的模式，rustup / bun / homebrew 都配这个出口，不给等于逼谨慎用户放弃。
4. ~~同时给出两步写法~~ **已从页面移除**（2026-08-28，@sam 实机评审：「没有用」）。
   P1 曾实现并实测跑通（P1 卡有记录）；移除的只是**页面上的教学**，
   地址本身照旧下发脚本正文，想逐字校验的人 `curl -o` 存下来看仍然可行——
   只是页面不再把这套流程当作一个功能来展示。
   原理由里那条「审阅链接给的是模板，下发的是模板 + 注入的密钥」的差异**依然真实**，
   若日后有用户真的问出这个差异，恢复的成本也就是几行 UI。
5. **发令牌的接口需已登录会话**，且令牌只能换到**该用户自己的** key。
6. **`~/.deeprouter/env.sh` 权限设为 `600`** —— 它里面是明文密钥，不能让同机其他用户读到。
7. **脚本不上报任何东西。** 本期不做遥测——它运行在用户机器上、持有用户密钥，任何回传都需要单独的隐私评审。
8. **不引入运行时依赖。** 不要求 Node / Python / 任何需要用户先安装的东西（Q3）。一个「让配置变简单」的工具，自己先要求装一套运行时就自相矛盾了。

---

## 6. 失败态（都要人话，不裸露 HTTP 码）

| 情况 | 输出 |
|---|---|
| 一个支持的工具都没检测到 | 明确列出支持哪四个，并给出安装入口。**绝不静默成功** |
| **所有已装工具都因已有付费登录被跳过** | 逐个说明为什么跳过、以及 `--force` 怎么用。**这不是失败** —— 但绝不能打印「配置完成」了事，否则用户会以为配好了 |
| 配置文件解析失败（坏 JSON / 坏 TOML） | 中止**该工具**，指出文件路径，建议手工检查。**不动那个文件**，其余工具继续 |
| 认不出 shell 类型 | 跳过 shell 写入，**明确告诉用户需要手动加哪一行**，其余工具照常配好 |
| 文件只读 / 无权限 | 说清是哪个文件、需要什么权限 |
| 令牌过期或已用过 | 「这条命令已失效，回密钥页重新复制一条」 |
| 验证 401 | 「密钥无效，请重新生成」 |
| 验证 402 / 余额不足 | 「余额不足，请先充值」+ 充值链接 |
| **验证 403 `预扣费额度失败`** | 🔴 **也是余额不足**，不是权限问题。网关会给出「剩余 $X，需要预扣 $Y」，**把这两个数字原样报给用户** + 充值链接。<br/>⚠️ **Claude Code 自己会把这个 403 显示成 `Please run /login`**（§0.1 F5）—— 用户会反复重登而不是去充值。**这正是脚本必须自己发验证请求并翻译错误的理由**：工具给的诊断是错的 |
| 验证 503 | 「模型暂时繁忙，稍后重试」（配置**已写好**，要说清楚这点） |
| 某一种协议验证失败、其余通过 | 逐条报，**不要一条失败就说整体失败** |
| 无网络 | 「连不上 DeepRouter，检查网络后重试」 |

失败态文案沿用 `key-setup-guide-prd.md` §5 的既有映射，不另造一套。

---

## 7. 验收标准

> ### ⚠️ 怎么测：不要拿自己的全局登录去试
>
> 做这张卡的人**很可能自己就是 Claude Code 用户，而且是订阅登录**（`~/.claude/.credentials.json` 里的 `claudeAiOauth`，`subscriptionType: max` / `pro`），**根本没有 API key**。把 `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` 写进全局 `~/.claude/settings.json`，会顶掉他自己的订阅认证——测试期间他本人的 Claude Code 就在按 token 花钱，而已付费的订阅闲置。
>
> > 🔴 **本条已按 F1/F8 更新。** 原文写「测试一律用项目级 `.claude/settings.json`」——
> 那条路**根本不生效**（F1：项目级 `env` 块不会让 Claude Code 跳过登录），而且脚本改走环境变量后
> 也不再写这个文件了。
>
> **正确做法：用进程级环境变量。** 只给被测进程设 `ANTHROPIC_BASE_URL` / `ANTHROPIC_AUTH_TOKEN` /
> `ANTHROPIC_MODEL`，**不写注册表、不写 `~/.claude/`**，进程退出即无痕：
>
> ```bash
> ANTHROPIC_BASE_URL=... ANTHROPIC_AUTH_TOKEN=... claude -p "hi"
> ```
>
> 已用这个方式在 `.co` 上跑通一次（F20 就是这么测出来的），**全程未碰
> `~/.claude/.credentials.json`**。只有验「持久化」本身（§4.2 写注册表 → 新终端）才必须真写，
> 那一步走 §4.6 的卸载回滚。
>
> 📌 顺带说明本 PRD 为什么反复强调「合并、绝不覆盖」：一个真实开发者的 `~/.claude/settings.json` 通常有数千字节、十来个顶层键（`permissions` / `hooks` / `theme` / `autoMemory` …）且**没有 `env` 块**。脚本要凭空加一个 `env` 块，同时把其余每一个键逐字留住。§4.3 那条 🔴 保护的就是这个文件。
>
> 卸载（§4.6）在这里是刚需而不是礼貌：测的人必须能一条命令回到测试之前的状态。

> ### 🔴 在哪测：本地起一套，`.co` 只做最终验收
>
> 本 PRD 有相当一部分验收**在生产上没法安全制造**（要一个只挂 TTS 模型的目录、
> 一个余额刚好卡在中间的账号、一张带白名单的令牌…）。**本地起一套就都能造。**
>
> ```bash
> cd deeprouter && docker compose -f docker-compose.smart-router.yml up -d --build
> node ../scripts/mock-openai/server.mjs        # 假上游，:9999，dev 库渠道 #1 已指向它
> ```
>
> | | 测什么 |
> |---|---|
> | **本地** | 网关逻辑 · **边界场景** · 脚本行为 · 改完的回归 |
> | **`.co`** | 最终验收 —— 真实目录、真实上游、真实计费 |
>
> 🔴 **这几条只能在本地做**（生产上造不出对应的部署状态）：
>
> - 「不能对话的模型被排除」→ 需要一个挂着 TTS / audio 模型的目录
> - 「`deeprouter-auto` 不可用时改用真实模型」→ 需要一个只挂 Claude 渠道的部署
> - 「探测的 `max_tokens` 与工具实发相当」（F20）→ 需要**余额刚够最小请求、不够真实对话**的账号
> - 「三种 4xx 分开处理」「§6 失败态逐行走一遍」→ 每一种都要人为造出来
> - P1 的令牌白名单约束 → 需要同时有带白名单和不带白名单的两张令牌
>
> 📌 **mock 上游还解决一类线上解决不了的问题：网关到底往上游发了什么。**
> `top_k` 那个 bug 在线上只能从 OpenAI 回的 `400 Unrecognized argument: top_k`
> **反推**；在 mock 里打印一下请求体就**直接看见**，改完再看一遍就知道修对没有。


**页面**

- [ ] 密钥页出现「一键配置」区块，命令按操作系统自动切换（Windows 显示 `irm`，其余显示 `curl`）。
- [ ] Windows 区块明确提示用 PowerShell / 终端，并**点名不要用 cmd**。

**各工具**

- [ ] 干净 macOS，只装 Claude Code：配好、备份生成、Anthropic 协议验证通过、`claude` 直接可用。
- [ ] 干净 Windows（PowerShell），只装 Claude Code：同上。
- [ ] 只装 Codex：`config.toml` 写好、`~/.deeprouter/env.sh` 生成、`.zshrc` 多了**一行**引用、新开终端后 `codex` 可用。
- [ ] 🔴 **Codex 的两个分支各跑一次（Q7，本卡原本最大的工程风险）**：
      ① `~/.codex/config.toml` **不存在** → 直接写 `config.toml`，用户敲 `codex` 即可；
      ② `config.toml` **已存在** → 只写 `deeprouter.config.toml`，**那份原文件逐字未变**，且**输出里明确告知要敲 `codex --profile deeprouter`**。
      ⚠️ 不告知第二种情况的用户会以为没配上 —— 机制已由 §0.1 V10 实测，这里验的是脚本。
- [ ] 只装 Gemini CLI：Gemini 协议验证通过、新开终端后 `gemini` 可用。
- [ ] 🔴 **Gemini 的 `~/.gemini/settings.json` 确实被写入**，且是**嵌套键** `security.auth.selectedType = "gemini-api-key"`（§0.1 F10）。不写它，`gemini` 报 `Invalid auth method selected.` 且**一个请求都不发** —— 而脚本会以为自己配成功了。
- [ ] 只装 OpenCode：`~/.config/opencode/opencode.json` 写好，`opencode models` 列出 DeepRouter 的模型，**无需重开终端**即可用。
- [ ] ⚠️ **Gemini 的模型名落点已查清**（§0.1 F12 未解：`GEMINI_MODEL` 实测无效，`-m` 标志脚本用不上）。**这是四个工具里唯一一个没有结论的点，不能带着它开工。**
- [ ] 四个都装：四个都配好，**四种协议分别**验证通过。

**用户选择（可控性）**

- [ ] 🔴 **没勾选的工具，脚本碰都不碰** —— 预先把四个工具的配置文件都放上可识别内容，只勾一个跑完之后，另外三个**逐字未变**。
- [ ] 页面命令下方显示「这条命令将配置：…」，与勾选状态一致。
- [ ] 改动勾选后，命令随之更新（令牌重新签发）。
- [ ] **勾了但本机没装** → 明确告知「你选了 X，但本机没找到」，不静默略过。
- [ ] 🔴 **「只有配置目录、没有可执行文件」不算装了（§0.1 F9）**：造一个只有 `~/.codex/` 而 `codex` 不在 PATH 上的环境（**装一个 ChatGPT 桌面版就是这个状态**），脚本应告知「找到配置目录但没找到 `codex` 命令」并**跳过**，而不是给它写配置、再让用户敲一个不存在的命令。
- [ ] 🔴 **反向也要成立**：工具装了但**配置目录不存在**时照样识别并配好（Gemini CLI 刚装完就是这个状态，`~/.gemini` 要跑过才生成）。
- [ ] 一个都没勾时，不给出可执行的命令（或明确提示先选一个）。

**不误伤已付费的登录**

- [ ] 🔴 **一台有 Claude 订阅登录的机器上跑脚本，`~/.claude/` 下的认证与 `settings.json` 完全不被改动**，输出里说明了为什么跳过。**这是唯一一条「脚本正常工作却让用户损失金钱」的路径。**
- [ ] `--force claude-code` 时才配置它，且仍然备份 + 合并（不是覆盖）。
- [ ] `--only codex` 时只动 Codex，其余一个不碰。
- [ ] 所有已装工具都被跳过时，**不打印「配置完成」**，而是逐个说明原因与 `--force` 用法。
- [ ] 跳过的工具**不发验证请求**，也不计为验证失败。
- [ ] 🔴 **另外三个工具的登录态也要各自核实**：本 PRD 只查证了 Claude Code 的信号（`~/.claude/.credentials.json`，§4.2.1）。Codex 有 ChatGPT 登录、Gemini 有 Google 登录、OpenCode 有自己的 `providers` 凭据 —— **三个的信号在哪、能不能光看文件存在性判断，都还没查**。同样遵守「存在即跳过，不读内容」。

**不硬编码 / 不照搬（POC 推翻的四条，§0.1）**

- [ ] 🔴 **验收在正式部署 `deeprouter.co` 上做**，不是 `deep-router.com`（§0.1）。两套库不通，在非正式那套上验收通过**不代表**真实用户能用。
- [x] 模型清单与计费链路在 `.co` 上重测过（§0.1 R1/R2，2026-08-26）—— 两套部署的模型目录**确实不同**（`.co` 有 Opus 4.7 thinking / GPT-4o mini），且 `GPT-4o mini` 完整跑通一次调用。
- [x] F3 的 `deeprouter-auto` 已在 `.co` 上重测（§0.1 R4，2026-08-26）——曾 100% 失败（令牌白名单）。✅ **已修复**：ADR-0007，`e13548a8`，本地 stash 前后对照 403→200，卡在 eval，待合并部署。
- [ ] 🔴 **脚本发出去的令牌能真的用 `deeprouter-auto`**（§0.1 R4）——拿脚本实际拿到的令牌发一次 `deeprouter-auto`，**返回 200**。✅ 白名单冲突已修复（ADR-0007，`e13548a8`，eval）——P1 **不必**再决定「不带白名单」；本条在修复合并部署后的 `.co` 上实测。
- [ ] 🔴 **Base URL 全程来自令牌接口**，脚本、前端、文案里搜不到任何硬编码的 `https://api.deeprouter.co`。用两个不同部署的令牌各跑一次，各自指向正确的主机。
- [ ] 🔴 **模型名经探测后决定**：在一个 `deeprouter-auto` 不可用的部署上跑，脚本能自动改用 `/v1/models` 里的真实模型，并在输出里说明。
- [ ] 🔴 **探测与验证请求用的是目录里最便宜的模型**（§4.3、§0.1 F16）—— 在一个余额只够小模型的账号上跑，脚本能配完并验证成功，**不会因为拿贵模型探测而 403**。
- [ ] 🔴 **探测把三种 4xx 分开处理**（§4.3、§0.1 F19）：`该令牌无权访问模型` / `预扣费额度失败` / `404 does not exist` —— 三种都**自动换下一个候选**，只有候选全试完才提示用户，且提示里说清是余额还是权限。
- [ ] 🔴 **不能对话的模型被排除**：在一个 `/v1/models` 含 `gpt-4o-mini-tts` / `gpt-4o-audio-preview` 的部署上跑，脚本**不会**选中它们。
      🔴 单列是因为 `gpt-4o-mini-tts` 返回的是 **403 预扣费 $0.36** 而非「模型类型不对」，会被误报成余额不足，**把用户送去为一个永远不能对话的模型充值**。
- [ ] 🔴 **按 `supported_endpoint_types` 给每个工具挑模型**：Claude Code 拿含 `anthropic` 的，OpenCode / Codex / **Gemini CLI** 拿含 `openai` 的。⚠️ **没有任何模型声明 `gemini`**。
- [ ] 🔴 **写进配置的模型是实发请求确认过 200 的**，不是查元数据算出来的。
- [ ] 🔴 **探测用的 `max_tokens` 与工具实发相当**（§0.1 F20）——在一个余额刚够最小请求、不够真实对话的账号上跑，脚本**不会**报「配置成功」。
      🔴 实测:同一模型同一账号,`max_tokens` 16 → 200、4096 → 403、**Claude Code 实发 → 403 预扣 $0.16**。用最小请求探测会写下一份「验证通过、发一句话就 403」的配置。
- [ ] 🔴 **Claude Code 走环境变量**：配完后**完全关掉终端再新开一个**，`claude` 首屏显示 **`API Usage Billing`**。
      🔴 **用这个字样作判据，不要用「没问登录方式」** —— 后者是阴性信号（没弹可能只是版本变了），而 `API Usage Billing` 是 **Claude Code 自己声明了走 API 计费而非订阅**（§0.1 V11 实测）。
- [ ] 🔴 **Windows 在默认 `Restricted` 下可用**（§0.1 F8）：把测试机的 execution policy 恢复成默认（`CurrentUser` 与 `LocalMachine` 均 `Undefined`）再跑一遍全流程 —— 配置生效，且**开终端不出现任何红色报错**。
      ⚠️ 开发机通常已被改成 `RemoteSigned`，**在开发机上测不出这个问题**，必须显式恢复默认或用干净机器。
- [ ] 🔴 **Windows 卸载按变量名清除**：卸载后 `HKCU\Environment` 里我们设的变量消失，而用户自己原有的变量**逐个还在**。（机制已由 POC #3 V8 验过，此处验的是真脚本。）
- [ ] 🔴 **不要求管理员权限**：全程在非提权终端里跑通（V7 已证机制可行，此处验脚本没有多余的提权要求）。
- [ ] 🔴 **Windows 全链路在无订阅的机器/账号上跑一次**：真脚本 → 关终端 → 新开 → `claude` 直接进输入框。
      ⚠️ **不可在开发者自己的日常机器上做** —— 会顶掉本人的 Claude 订阅（§4.2.1）。这条是**三轮 POC 都刻意绕开**的一段，必须补。
- [ ] 「重开终端」的引导文案明确写成「**完全关闭当前窗口后重新打开**」，而不是「重开终端」—— 从当前窗口里再启动一个不算（§3 D2）。

**安全与正确性（错了不会报错的那些）**

- [ ] 🔴 **原有配置保留 —— 四个文件都要验**：`~/.claude/settings.json`、`~/.codex/config.toml`、🔴 `~/.gemini/settings.json`、🔴 `~/.config/opencode/opencode.json`，各预先放入无关设置，跑完后逐字还在，且备份文件存在。
      ⚠️ **后两个是新增的**：Gemini 原本被归为「纯环境变量、不写文件」（§0.1 F10 推翻），OpenCode 则一直漏在这条验收外。
- [ ] 🔴 **坏配置不被破坏**：预先写入语法错误的 JSON 与 TOML，脚本跳过该工具并保留原文件不变，其余工具仍然配好。
- [ ] 🔴 **fish 语法**：在 fish 下跑一次，`config.fish` 里是 `source .../env.fish`，`env.fish` 里是 `set -x`，**新开 fish 终端无任何语法错误**。
- [ ] 🔴 **四个 Base URL 各不相同**且都正确：Claude Code **无** `/v1` · OpenCode/Codex **带** `/v1` · Gemini 🔴 **也不带**（不写 `/v1beta`，CLI 自己拼，§0.1 F11）。
- [ ] 🔴 **Codex 实际打的是 `/v1/responses`**（不是 `/v1/chat/completions`），且 `wire_api = "responses"`（F13/F14）。
- [ ] **幂等**：连跑两次，所有配置文件内容一致，shell 配置里**仍然只有一行**引用。
- [ ] 原 shell 配置文件结尾无换行时，追加不会与最后一行粘连。
- [ ] `~/.deeprouter/env.sh` 权限为 `600`。
- [ ] 令牌用过一次后失效；超过 15 分钟后失效。
- [ ] 密钥不出现在命令、URL 或终端回显中。

**卸载**

- [ ] 🔴 **跑完卸载，机器回到跑脚本之前的状态**：四个配置文件与 shell 配置逐字等于安装前（用安装前的副本做 diff 比对），`~/.deeprouter/` 已删除。**这条比卸载功能本身更重要——它是唯一能证明「备份 + 还原」链路完整的测试。**
- [ ] **脚本新建的配置文件被删除，而不是留下一个空壳**（`pre_existing: false` 的分支）。
- [ ] **安装两次再卸载**，仍然还原到**第一次安装之前**的状态（验证 `original_backup` 未被第二次安装覆盖）。
- [ ] shell 配置里**只有那一行被删掉**，用户自己的其余内容逐字保留。
- [ ] 从未装过时跑卸载：明确告知没检测到，**不做任何猜测性删除**。
- [ ] 🔴 **清单 `~/.deeprouter/installed.json` 内容正确**（§4.6）：逐工具记了 `pre_existing`、`original_backup` 路径，**以及 Windows 上我们设过的每一个环境变量名**。卸载完全依赖它 —— Windows 不写文件，没有清单就无从下手（§3 D2）。

**输出**

- [ ] 明确区分「立即生效」（🔴 **只剩 OpenCode**）与「需重开终端」（Claude Code / Codex / Gemini）—— F1 之后 Claude Code 已从前者挪到后者。
- [ ] 🔴 **提示首次启动的手动步骤**。已实测到的两项：**Claude Code 会先问「Is this a project you trust?」**（v2.1.246，需回车确认，脚本做不了）、Gemini CLI 的「选 API key 而不是 Google 登录」。
      ⚠️ 早先版本写的「按 Esc 跳过 Anthropic 登录」已不适用 —— 环境变量配好后根本不会弹登录。**单一个会话内 Claude Code 就从 v2.1.177 跑到了 v2.1.246** —— 这正是“以当前版本实测为准”的实例。**以在当前版本上实测为准**——这些引导流程由工具方决定、会随版本变化，验收时必须真的装一次新的、跑一次，而不是照抄本 PRD 或指南里的描述。
- [ ] **用一个完全没有 Anthropic 账号的环境跑通**：从零装 Claude Code → 跑脚本 → `claude` 能正常对话。这条同时验证了产品的核心主张（没有海外信用卡也能用上 Claude Code），是最接近真实新用户的一条路径。
- [ ] 🔴 **告知 claude.ai connectors 会被禁用**（§0.1 F15）—— 配上之后 Claude Code 底部常驻 `claude.ai connectors are disabled because ...` 。**这是本方案目前已知的唯一一项功能损失**，不主动说，用户既不知道失去了什么、也不知道是我们造成的。卸载后应恢复（需验）。
- [ ] 安装成功的输出里带卸载命令。
- [ ] 没装任何支持工具时给出明确提示而非静默退出。

**失败态文案（§6 那张表，逐行验）**

> ⚠️ **§6 定义了 11 种失败态，此前一条验收都没有。** 而 §6 的整个立论是「工具自己给的诊断可能是错的」（F5：Claude Code 把余额不足报成 `Please run /login`）—— 如果脚本的文案没人验，这份 PRD 最核心的主张就落空了。

- [ ] 🔴 **逐行走一遍 §6 的表**，每种情况都能人为造出来并看到对应文案：令牌过期 / 401 / 402 / **403 预扣费额度失败** / 503 / 坏 JSON / 坏 TOML / 认不出 shell / 文件只读 / 无网络 / 一个工具都没检测到。
- [ ] 🔴 **403 `预扣费额度失败` 被翻译成「余额不足」**，并**原样带出网关给的「剩余 $X，需要预扣 $Y」两个数字** + 充值链接。⚠️ 这是唯一一个 HTTP 码与语义不符的，也是 F5 的实证来源。
- [ ] **任何一条失败态里都不出现裸的 HTTP 状态码**。
- [ ] **某一种协议失败、其余通过时逐条报**，不因一条失败就说整体失败。
- [ ] 503 的文案说清**配置已经写好了**，只是模型忙。
- [ ] 文案与 `key-setup-guide-prd.md` §5 的既有映射一致，**没有另造一套**。

**安全（§5 八条硬约束，逐条验）**

- [ ] 🔴 **令牌只能换到「该用户自己的」key**（§5.5）：拿 A 用户的登录会话签的令牌，**换不到** B 用户的 key。这是唯一一条越权路径，**必须有单测**。
- [ ] 🔴 **发令牌的接口要求已登录会话** —— 未登录直接调用被拒。
- [ ] 「先看看这个脚本」链接可点、指向 `deeprouter/internal/connect/` 的模板源文件（§5.3）。
- [ ] 🔴 **脚本不上报任何东西**（§5.7）：抓一次网络，除了「换令牌」与「验证请求」之外**没有任何出站连接**。
- [ ] 🔴 **不引入运行时依赖**（§5.8）：在一台**没有 Node、没有 Python** 的干净机器上跑通两个平台。⚠️ 开发机通常都装了，**测不出来** —— 与 U3 同一类问题。
- [ ] 全程仅 HTTPS，且只连自有域名。

**测试**

- [ ] 后端令牌逻辑带单测（`rules/unit-tests.md`）。
- [ ] 两份脚本的行为一致性有测试覆盖（同一套用例跑两个平台）。

### Track B1 —— 深链（§4.7 B1）

- [ ] 🔴 **深链入口出现在密钥页主视图**，不再只藏在行末 `⋯` 菜单里。
- [ ] 🔴 **真机点通一次**：点 Cherry Studio 那一项 → 应用被拉起 → provider 已建好、Base URL 与密钥已填 → **发一条消息，DeepRouter 后台看得到这次调用**。（"应用被拉起"不算成功，要看到调用记录。）
- [ ] 预设列表来自 `/api/status`，**不硬编码** —— 运营在后台增删预设，页面跟着变。
- [ ] 点击前有一句说明会发生什么；**未安装该应用时有可见兜底**（指向 B2 的指南），不是静默无反应。
- [ ] 深链里的密钥**只在点击那一刻解析**，不在页面上以明文长期驻留。

### Track B2 —— 文档（§4.7 B2）

- [ ] 🔴 **动笔前先处理掉旧拷贝**：`deeprouter-ai/docs/integrations/` 加横幅指向真相源，或删除并留一行面包屑（`rules/docs-currency.md`）。不做这步，下一个人还会改错那一份，而且不会报错。
- [ ] 所有内容改动只落在 `deeprouter/web/default/public/docs/integrations/`（Q8 定案的真相源）。
- [ ] `/llms.txt` 可访问，列出全部工具指南的地址。
- [ ] 每篇指南的 `.md` 原文能被直接 GET 到（不需要跑 SPA），**中英两版都是**。
- [ ] 事实块在 `<slug>.md` 与 `<slug>.zh.md` **两个文件里都存在**且值一致。
- [ ] 事实块里的 Base URL / model / 认证头 **逐个对活网关验证过** —— 同 `CLAUDE.md` §0 rule 3；一份写着跑不通的值的资料比没有资料更糟。
- [ ] 🔴 **Cherry Studio 的尾斜杠规则出现在它的事实块里**，且措辞不引人写出错误的那一种。
- [ ] **端到端验证**：把「我要在 Cherry Studio 里用 DeepRouter」这类问题连同指南链接丢给至少两个不同的 AI，它们给出的 Base URL **能实际配通**。这是 Track B 唯一真正的验收——不是「文档存在」，是「AI 拿它答对了」。
- [ ] 🔴 **密钥页那段求助文本里不含密钥**，用户复制后贴给第三方 AI 不泄露任何凭据。

### Track B3 —— 指南补上一键路径（§4.7 B3）

> B1 修的是密钥页的发现性；B3 修的是**指南本身在教难的那条路，且从不提简单那条存在**。两者互不替代 —— 用户被指过去的地方是指南。

- [ ] 🔴 **四份指南都在「操作步骤」之前先给一键路径**：`cherry-studio` · `cc-switch` · `lobehub` · `opencat`，**中英各一份，共 8 个文件**。
- [ ] 🔴 **手动步骤保留，降级为「或者手动配置」**，不是删掉 —— 深链失败、应用版本不认、企业环境禁用自定义协议时，它是唯一的路。
- [ ] 🔴 **逐个真机点通**，不是照抄 Cherry Studio 的写法。四个应用的深链类型**并不相同**（Cherry Studio 深链全填 · CC Switch 与 OpenCat 走专用通道 · Lobe Chat 是 URL 参数），点击后看到的界面也不同。
      ⚠️ **OpenCat 是 macOS / iOS 应用** —— 这一项需要 Mac，与 §0.1 U7 是同一个依赖，**排期前先确认有人能验**。
- [ ] 🔴 **点击前一句话预告会显示成「New API」而不是 DeepRouter**（Cherry Studio 实测如此，`id: 'new-api'` 匹配的是它的内置预设，上游命名，我方改不了）。不写这句，用户会以为点错了。
- [ ] **写明深链失败时怎么办** —— 应用没装 / 协议未注册 / 版本不认，都要能落回同一篇里的手动步骤。
- [ ] ✅ **域名核对（已从硬依赖降为确认）**：这 8 个文件里的 `api.deeprouter.co` **现在是对的**（§0.1 域名结论已订正）。动笔前对一下 `deeprouter-ai/deploy/README.md` §「There is a second deployment」，确认没又变就行，**不必等那张卡完成**。
- [ ] **确认那 5 个「有一键能力却没有任何指南」的应用已另立卡**：`AionUI` · `DeepChat` · `AI as Workspace` · `AMA 问天` · `流畅阅读（FluentRead）`。其中 AionUI 与 DeepChat 是 `DEEPLINK-FULL`，与 Cherry Studio 同级别。**不属于 B3 的范围，但它比 B3 更大 —— 不记下来就会被忘掉。**

---

## 8. 开放问题

**全部关闭。** 决定都已落进 §4 规格；这里只留**为什么没选另一条路** —— 那是规格里看不到、
而下一个人最容易重新提一遍的东西。

| # | 结论 | 为什么不是另一条路（🔴 = 踩过） |
|---|---|---|
| **Q1** | 写哪个模型名 —— **分工具不同** | 🔴 **原答案已被 F17/F19 取代**，以 §4.3 为准。原表写「Gemini CLI 用具体 Gemini 模型名」，而 `.co` 根本没有 Gemini 模型 |
| **Q2** | OpenCode 路径 —— 跑 `opencode debug paths` **问工具要**，失败才回落 `~/.config/opencode/` | 硬编码任何路径都会对改过 `XDG_CONFIG_HOME` 的人失效。⚠️ `%APPDATA%\opencode\` 是**插件数据不是主配置**，社区已有人因此配错（NoeFabris/opencode-antigravity-auth#251）。📌 通则：**能问工具要路径，就不要硬编码** |
| **Q3** | 交付方式 —— 维持 `curl \| sh` / `irm \| iex`，**不用 npx** | npx 能一条命令通吃双平台，但前提「用户必然有 Node」**在开发机上当场证伪**：`claude` 装在 `~/.local/bin/`，npm 全局前缀是 `%APPDATA%\npm`，不匹配。指南写的只是**其中一种**装法。一个「让配置变简单」的工具不该先要求装一套运行时。代价与对策见 §2.1 / §5 |
| **Q4** | 脚本源码放 `deeprouter/internal/connect/` | 与它调用的 `/api/connect/exchange` **同仓保证版本一致**；符合 `AGENTS.md` 的 `internal/` 规矩；fork 是公开仓库，§5 的审阅链接直接指 GitHub 即可。⚠️ **链接给的是模板，下发的是模板+密钥**，页面文案要说清 |
| **Q5** | Gemini —— **不赌 `.env` 文件** | 向上搜索**遇 `.git` 目录即停**，而开发者基本都在 git 仓库里跑；官方文档对家目录回落有**两处互相矛盾**的说法。🔴 **但当时「走 shell 就够了」也是错的** —— 还必须写 `settings.json`（F10）。📌 教训：**文档不可靠时，换一个同样出自该文档的方案并不降低风险，只有实测能** |
| **Q6** | Codex —— 用 `env_key`，不写 key 进 `config.toml` | `experimental_bearer_token` 在能访问到的官方文档里**无法证实**，且字段名自带 `experimental_`。官方明确写着别把 key 放进 `~/.codex/auth.json` |
| **Q7** | TOML —— **不合并** | Codex 支持**独立 profile 文件** `~/.codex/<name>.config.toml`（顶层键，非 `[profiles.x]` 段）。已有 `config.toml` 的用户就写我们自己那份，**碰都不碰他的**。代价：要带 `--profile deeprouter`（官方无「设默认 profile」的配置项，躲不掉），换掉的是本卡最大的工程风险。**未采用** `[model_providers.<id>.auth].command`：跨平台不是同一个东西，等于再开一条双平台分支 |
| **Q8** | 指南真相源 = **`deeprouter/web/default/public/docs/integrations/`**（49 个文件，含 `.zh.md`） | 它**唯一有中文版**，而站点是双语的。`deeprouter-ai/docs/integrations/` 那 25 个是纯英文，中文读者看到的内容根本不在里面。这等于把 `integrations-prd.md` §5 悬了两个月的选择**判给 Option A**，且已是既成事实。⚠️ **那 25 个文件必须在 Track B 动笔前加横幅或删除**（`rules/docs-currency.md`），否则下一个人还会改错那份 |

> 📌 **Q2 / Q5 / Q7 是同一个模式赢了三次:写我们自己的文件、问工具要答案,
> 不往别人的文件里塞东西。** 这也是 §3 D2 `~/.deeprouter/env.sh` 的由来。

> 🔴 **一条影响估时的净结论:shell 写入机制不是可选项,是必需项** —— Codex 与 Gemini 都要用它。
> §3 D2 第 3 级因此从「兜底」变成「这两个工具的正式方案」。

## 9. 被否决的方案

| 方案 | 为什么否 |
|---|---|
| **网页上一个按钮直接配好——写用户的配置文件** | 浏览器沙箱不允许网页写硬盘。这是**终端工具**那半边必须有「用户自己跑一次」的原因。<br/>⚠️ **但这条不适用于 GUI 应用**：协议深链（`cherrystudio://…`）根本不需要写硬盘，由应用自己接管。本 PRD 的早期版本据此断言「网页一键配置做不到」，**那是错的** —— 对 Cherry Studio / DeepChat / AionUI 恰恰做得到，而且代码已经在仓库里（见 §4.7 B1） |
| **把 export 直接追加进用户的 shell 配置文件** | 不是不安全，是**不幂等**：重复运行越堆越多、改 key 又要动他的文件、卸载要去他文件里找我们那几行。改用「一行引用 + 我们自己的文件」全部解决（§3 D2） |
| **逆向写 Cherry Studio / Chatbox 等 Electron 应用的内部存储** | 格式无文档、不承诺兼容、应用运行时写入会损坏；LobeChat 是网页应用，配置在浏览器里，任何本地程序都碰不到 |
| **密钥直接放进 URL** | 会留在 shell history、终端滚动区、录屏与截图 |
| **桌面安装器（`.exe` / `.dmg`）** | 代码签名约 $400/年、双平台、自动更新、「装不上」长尾客服——按月计的投入。**应当先用本方案验证需求真实存在**，再决定是否升级 |
| **在脚本里上报使用情况** | 它持有用户密钥、跑在用户机器上，任何回传都需单独隐私评审。本期一律不做 |
| **让 GUI 应用里的 AI 自己完成配置**（用户把 Base URL + key 发进聊天框，请它配好） | **循环依赖**：要跟聊天框里的 AI 说话，AI 得先能工作；而它不能工作的原因正是还没配置。**没配好之前，那个聊天框里没有 AI。**<br/>退一步，即使用户已配好别的服务商、AI 能回话，它**仍然改不了应用自己的设置** —— 聊天框里的模型只是在生成文字，没有「修改本应用 provider 配置」这个能力。聊天类应用也不会去加：那等于让对话内容能改自己的付费配置，是个明显的攻击面。<br/>→ 可行的是反过来做：**让文档对 AI 可读**，见 §4.7 |

---

## 10. 工程量估计

| 项 | h |
|---|---|
| 后端：令牌签发（**含勾选清单**）+ `GET /i/{token}` 下发脚本 + 单测 | 3.5 |
| **令牌与 `deeprouter-auto` 的白名单冲突：拍板 + 验证（§0.1 R4 新增）** | **1** |
| 查证 Q5 / Q6 / Q2（三个工具的真实落点） | 1 |
| POSIX sh：检测 / 备份 / JSON 合并 / 验证 / 输出 | 2.5 |
| **探测 Base URL 与可用模型（不硬编码，§0.1 F2/F3）** | **1.5** |
| **按优先级试模型 + 实发确认 + 分开三种 4xx（§0.1 F16/F19）** | **2.5** |
| Codex：两分支写文件（新建 / 独立 profile），**无需 TOML 解析器** | 1 |
| **Gemini：`settings.json` 嵌套键写入 + 合并保护 + 备份（§0.1 F10 新增）** | **1** |
| shell 检测 + `env.sh` 机制 + 三套语法（含 fish） | 2 |
| 已有付费登录的检测 + `--only` / `--force` 参数 | 1 |
| PowerShell：行为对齐（**改走注册表，不写 `$PROFILE`**，§0.1 F8） | 2.5 |
| **清单 + 卸载（两个平台）** | 2 |
| 前端：密钥页区块 + **工具勾选 UI** + 按 UA 切命令 + Windows 提示 | 3 |
| 双平台实机验证（含 fish、四种协议、卸载还原、§7 的破坏性用例、**默认 `Restricted` 的干净 Windows**） | 4.5 |
| — **以上为 Track A** — | |
| **Track B1：把深链入口提到密钥页主视图 + 真机点通验证** | **2** |
| Track B2：旧拷贝加横幅/删除 + `/llms.txt` + `.md` 原文可直取 | 2 |
| Track B2：加事实块 —— **中英各一份，共约 46 个文件**（值只需验证一次，但要插两遍） | 4 |
| Track B2：密钥页的「求助文本」区块（不含密钥） | 1.5 |
| **Track B3：4 份指南 × 中英插入「一键配置（推荐）」，手动步骤降级** | **1.5** |
| **Track B3：逐个真机点通验证（Cherry Studio 已验，剩 CC Switch / Lobe Chat / OpenCat）** | **1** |
| Track B2：端到端验证（丢给两个不同 AI，看它们答得对不对） | 1 |
| PRD 评审往返 + CHANGELOG（计入 **P1**） | 1 |
| **合计** | **43** |

> 📌 相对第一版 **+2h**，但构成不同了：Claude Code 改走环境变量后**省掉了它那份 JSON 合并**（−0.5），新增的探测逻辑（+1.5）与 Claude Code 的双平台 shell 验证（+0.5）更贵；F8 之后 Windows 写入反而**更简单**（注册表，不必管幂等与文件；−0.5），但要在**恢复成默认 execution policy 的机器**上重跑全流程（+1）。
>
> **这个数字现在比第一版可信得多** —— 第一版的 34h 建立在「写 `settings.json` 就行」这个**已被实测推翻**的假设上。两轮 POC 各推翻了它的一半（F1 推翻工具侧，F8 推翻 Windows 平台侧）。

按「全员用 Claude Code」的口径估（`deeprouter-ai/docs/adlc/TEAM.md`）：写代码的部分已压缩，**实机验证与评审往返按原价计**——这类「必须在真机上跑一遍」的成本不随工具变化，正是 P1 那张卡 8h→12h 超出的原因。

⚠️ **43h 超过任何一个人的周工时上限**（最高 @sam 30h），按 `rules/adlc.md` §2 必须拆卡。拆线都很干净：

> ✅ **已于 2026-08-26 拆卡完成（`/adlc-split`）** —— 下表是拆卡结果，不再是提案。
> 卡在 `deeprouter-ai/docs/adlc/tasks/one-click-*-task.md`，100 条验收逐字分配完毕，无遗漏无重复。

| 卡 | 内容 | h | 验收条数 |
|---|---|---|---|
| **P1** | 后端令牌 + 密钥页区块与勾选 UI（含 PRD 评审往返 + CHANGELOG） | **8.5** | **11** |
| **P2** | 两个平台的安装脚本 + 卸载 + 实机验证 | **21.5** | **68** |
| **P3** | 深链入口提到主视图 **+ 四份指南补上一键路径** | 4.5 | 12 |
| **P4** | 文档对 AI 可读：真相源 / `llms.txt` / 事实块 / 求助文本 | 8.5 | 9 |

**编号是依赖顺序,不是优先级**（`rules/adlc.md` §1）：P2 只依赖 P1 **接口的形状**，不依赖它的实现；
P3 / P4 与 Track A 完全无关，可并行。两者原本 `depends_on` 域名真相源卡，**域名结论订正后这层依赖已解除**
（那批文件里的 `api.deeprouter.co` 现在是对的）—— **P3 可以立刻开工**。

原提案里的 B-1 与 B-3 已**合并为 P3** —— 两者都是「让一键配置被看见」，同一个人、同一批文件区域。

⚠️ **P2 是唯一一张会撑破 3 天分支上限的**（18h / 60 条）。若真的太大，干净的切法是按
「**写进去**（检测/探测/合并/幂等）」与「**证明有效且能撤销**（验证/失败态/卸载/实机）」切开，
后者 `depends_on` 前者 —— 但那样会**追加成 P5**，不是把现有编号往后挪（编号一经分配即冻结）。

📌 **P3 只有 4.5h，却是唯一直接服务 `onboarding-v2-prd.md` §3 目标人群（非技术用户）的一张。若时间只够做一件事，做它** —— 注意这是**排期建议**，不是编号的含义：P1–P4 是依赖顺序，不是优先级。

> ✅ **这个建议的前置条件已经付清（2026-08-26）。** 它原本架在「深链点下去真的能配好」这个**未经实测**的判断上，而本 PRD 已有两次同强度判断被推翻（F1、F8）。所以开工前真的点了一次 —— **通了**，细节见 §4.7 B1。
>
> **2h 的估值站得住，而且偏保守**：实测发现模型可以由 Cherry Studio 自动拉取，用户不必去 Model Catalog 抄模型 ID —— 那一步原以为要额外的引导文案，现在不需要。
>
> ⚠️ 唯一新增的工作是**文案**：应用里显示的是「New API」不是 DeepRouter（上游预设命名，改不了），要在点击前一句话说明，否则用户会以为点错了。已计入 2h。

---

## 0.1 POC 实测记录（2026-08-26）

> 📌 **本节是全文 F / R / V / U 编号的出处** —— 每一条都是动手实测的记录，不是推理。
> 它原在文档开头，2026-08-27 移到文末作附录：读规格不需要先读它，但**规格与它冲突时以它为准**
> （它是实测，规格是写作）。**编号「0.1」保留不改** —— 任务卡与 LOG 里大量写着「§0.1 F<n>」，
> 改了编号那些引用就全部落空。

本 PRD 的第一版全部来自**读文档与读源码**。在写任何产品代码之前先跑了一个约 60 行的一次性 POC（Windows / Claude Code v2.1.177 / 真实生产网关），**结论与文档不一致的地方以此为准**。

### ✅ 已验证成立

| # | 结论 | 证据 |
|---|---|---|
| V1 | Claude Code **确实能被配置成走 DeepRouter** —— 核心机制成立 | `claude` 发出的请求由 DeepRouter 返回 `403 预扣费额度失败，用户剩余额度：$0.039042`。「预扣费额度」是 new-api 术语，request id 也是网关格式 —— 请求确实到了 DeepRouter，没走订阅 |
| V2 | Anthropic 原生 Base URL **不带 `/v1`** 是对的 | `POST https://<host>/v1/messages` 返回 401 而非 404 |
| V3 | `x-api-key` 与 `Authorization: Bearer` 均被网关接受 | 两者都进到了 token 校验 |
| V4 | 端到端计费链路通 | 一次成功调用后**账户余额下降** |
| **V5** | **Windows 注册表这条接缝通了** —— 写 `HKCU\Environment` → **关掉终端 → 新开一个** → 四个变量全部读到 | POC #3（2026-08-26）。刻意用 `DRPOC_*` 而非 `ANTHROPIC_*`：被测的是「注册表写入能否到达新终端」，操作系统不关心变量叫什么，而用真名会顶掉本人的订阅认证（§4.2.1） |
| **V6** | **值里的 `%` 不会被展开** —— `literal%USERPROFILE%literal` 原样读回 | `SetEnvironmentVariable` 存的是 `REG_SZ` 而非 `REG_EXPAND_SZ`。**这条本来是个「错了不报错」的坑**：若存成后者，含 `%` 的密钥会被静默改写。已排除 |
| **V7** | **不需要管理员权限** | 在非提权会话（`IsInRole(Administrator) = False`）里写 `HKCU\Environment` 成功。**脚本不必要求「以管理员身份运行」** —— 那一步会劝退大量目标用户 |
| **V8** | **卸载按名删除是干净的** | `-Clean` 后四个 `DRPOC_*` 消失，用户原有的 6 个变量（`GOPATH` / `OneDrive` / `OneDriveConsumer` / `Path` / `TEMP` / `TMP`）逐字还在 |
| **V11** | 🔴 **U2 全链路通过**（本 PRD 最后一段未验证的接缝）—— 真写注册表 `ANTHROPIC_*` → 完全关终端 → 新开 → `claude` **直接可用，不再问登录方式** | 🔴 判据不是“没问登录”，而是 **Claude Code 自己在首屏写的 `Opus 4.8 · API Usage Billing`** —— 它自己声明了走的是 API 计费而非订阅。随后 `hi` 返回网关的 403 预扣费错误 + request id。卸载后 4 个变量消失、原有 6 个逐字还在、`~/.claude/.credentials.json` 的 size+mtime **未变** |
| **V9** | **OpenCode 的 PRD 配置逐字可用** —— 四个工具里唯一一个一字不改就跑通的 | 写入 `~/.config/opencode/opencode.json` 后，`opencode models` 列出 `deeprouter/deeprouter-auto`；`opencode run` 返回网关的 `Invalid token` + request id（假密钥） |
| **V10** | **Codex 的独立 profile 文件方案（Q7）成立** —— 本卡原本**最大的工程风险**（在 sh 与 PowerShell 里手写 TOML 增删改）确实被消掉 | 造一份「用户原有的」`config.toml`，另写 `deeprouter.config.toml`，`codex exec --profile deeprouter` 输出 `provider: deeprouter`，请求到达网关，**用户原文件逐字未变** |

### 🔴 推翻或补充了本 PRD 原有规格的二十条

| # | 实测 | 原规格 | 影响 |
|---|---|---|---|
| **F1** | **只有真环境变量能让 Claude Code 跳过登录**；项目级 `.claude/settings.json` 的 `env` 块**不生效**——仍然弹出 "Select login method" | D2 把「写工具自己的配置文件」列为第 1 优先，Claude Code 走这条 | **D2 优先级反转**，见 §3 |
| **F2** | **Base URL 不是常量** —— 存在**两套独立部署**，各有各的数据库，**密钥不通用**（见下表） | 全文硬编码单一域名 | §4.1 令牌接口必须连 Base URL 一起返回。🔴 **本条的价值不在「哪个是正式的」，而在「这个问题的答案会变」** —— 它已经变过一次 |
| **F3** | **模型名不是常量** —— 两套部署的模型目录**实测不同**（R1）；且 `deeprouter-auto` 曾因令牌白名单在生产 **100% 失败**（R4，✅ 已修复 ADR-0007，待部署） | 全文写死 `deeprouter-auto` | §4.3 必须探测后再决定，探测第 1 步不能省 |
| **F16** | 🔴 **验证请求用哪个模型，决定它成不成功** —— 同一个账号、同一句 `hi`：`Claude Opus 4.7 (thinking)` 要求预扣 **$0.816000**（余额 $0.039954，403）；`GPT-4o mini` **直接通过**。预扣额 = 模型单价 × `max_tokens` 预授权，**与实际用量无关** | §4.3 的探测与 §4.4 的「四种协议各验一次」**都没规定用哪个模型** | 🔴 **探测与验证一律挑目录里最便宜的模型**，并把 `max_tokens` 压到最小。新用户余额小，拿贵模型探测会 403，而脚本会把「余额不够跑这个模型」误报成「配置失败」——机制是好的，却因为挑错了模型而报错。§6 的 403 文案要能区分这两件事 |
| **F17** | 🔴 **Gemini CLI 在 `.co` 上三条路全堵，但堵点在网关的协议转换，不在缺 Gemini 渠道** —— 配 `gemini-*` → 403（该部署确无 Gemini 渠道）；配 **`gpt-4o-mini` → 400 `Unrecognized request argument supplied: top_k`**；配 **`claude-sonnet-4-6` → 500 `convert_request_failed` / not implemented**。🔴 **而同样的 Gemini 端点用 curl 打 `gpt-4o-mini` 是 200** —— 差别只在 CLI 会发 `top_k` | §3 D1「Phase 1 = 四个工具全做」 | 🔴 **D1 不变，Gemini CLI 留在 Phase 1**。真正要修的是网关，另立卡 `gemini-protocol-conversion-task.md` |
| **F18** | **Gemini CLI 在 headless 下有信任门** —— `gemini -p` 直接报 `not running in a trusted directory`，一个请求都不发。要 `--skip-trust` 或 `GEMINI_CLI_TRUST_WORKSPACE=true` | §4.4「四种协议各验一次」默认脚本能直接跑 `gemini -p` 来验 | **脚本的验证步骤要带上这个标志**，否则 Gemini 那一条永远验不过，且报错和配置无关、极难排查 |
| **F19** | 🔴 **模型元数据不足以挑模型 —— §4.3 的探测流程按原文不可实现**。四条实测：① **`/v1/models` 没有任何价格字段**（只有 `id`/`object`/`created`/`owned_by`/`supported_endpoint_types`）；② 两个模型接口**互不包含**，4 个能用的模型（含实测 200 的 `gpt-4o-mini`、`claude-sonnet-4-6`）**任何地方都查不到价格**；③ `/v1/models` 里**混着根本不能对话的模型** —— `gpt-4o-mini-tts` 发对话返回 **403 预扣费 $0.36**（不是「模型类型不对」！），`gpt-4o-audio-preview` 返回 **404 does not exist**；④ 🔴 **`supported_endpoint_types` 里没有任何模型声明 `gemini`**（25 个 `[anthropic, openai]` + 4 个 `[openai]`）| §4.3「不通 → `GET /v1/models` 取模型选一个」+ F16「按价格升序试」 | 🔴 **§4.3 探测流程重写**：不能靠元数据挑，必须**实发请求确认**。③ 尤其危险 —— 它会被 F16 的新规则误报成「余额不足」，把用户送去为一个**永远不能对话的模型**充值 |
| **F20** | 🔴 **「探测通过」不等于「工具能用」—— 预扣费随 `max_tokens` 线性放大**。同一模型 `claude-sonnet-4-6`、同一账号($0.0396):`max_tokens` 16 / 1024 → **200**;4096 → 403 预扣 $0.0435;8192 → 403 预扣 $0.0583;**Claude Code 实发 → 403 预扣 $0.1600**(推算 ≈ 25k tokens)。**脚本用最小请求探测会拿到 200,写下配置,用户发第一句话就 403** | §4.3 第 3 步「逐个实发**最小**请求,第一个返回 200 的才写进配置」 | 🔴 **探测必须用与工具实发相当的 `max_tokens`**,或至少把「这个模型每次对话需要约 $X 预扣」告诉用户。最小请求探测**只能证明鉴权与模型可达,不能证明用户能真的对话** |
| **F15** | 🔴 **配上 DeepRouter 会关掉 claude.ai connectors** —— Claude Code 底部常驻一行警告：`claude.ai connectors are disabled because ANTHROPIC_API_KEY or another auth source is set and takes precedence over…` | 全文**从未提过这个代价** | 必须写进输出与指南。这是**本方案目前已知的唯一一项功能损失** —— 不说的话用户不会知道自己失去了什么，更不会知道是我们的脚本干的 |
| **F13** | **Codex 的 `wire_api = "chat"` 已被移除**（v0.149.1）—— 写了它 Codex **连配置都加载不了**：`Error loading config.toml: 'wire_api = "chat"' is no longer supported.` | §4.3 的 Codex TOML 写的就是 `wire_api = "chat"` | 改成 `"responses"` |
| **F14** | **Codex 走 `POST <base>/v1/responses`，不是 `/v1/chat/completions`** —— 实测如此；已确认网关支持该端点（无 key 探测返回 401 而非 404） | §4.4 验证表把 OpenCode 与 Codex 归在同一个 `/v1/chat/completions` 下 | **验证表拆开为两行** |
| **F10** | **Gemini CLI 不是纯环境变量** —— 必须写 `~/.gemini/settings.json`，且键是**嵌套的** `security.auth.selectedType = "gemini-api-key"`。不写它，`gemini -p` 直接报 `Invalid auth method selected.`，**一个请求都不发**。顶层 `selectedAuthType`（旧结构）在 v0.57 上无效；`GEMINI_DEFAULT_AUTH_TYPE` 环境变量**只在交互路径生效** | D2 把 Gemini 归入第 3 级「纯环境变量」 | **Gemini 也要写 JSON** → §4.3 的合并保护 / 备份 / 卸载还原全部适用于它，**这部分工程量原本没算** |
| **F11** | **`GOOGLE_GEMINI_BASE_URL` 不能带 `/v1beta`** —— CLI 自己会拼，带了就是 `POST /v1beta/v1beta/models/…`，网关返回 `Invalid URL`。**与 Cherry Studio 的 `/v1/v1` 同一个坑** | §4.3 写的是 `GOOGLE_GEMINI_BASE_URL=<base>/v1beta` | 去掉 `/v1beta`。**「Base URL 该不该带版本段」这件事每个工具的答案都不同，一个都不能想当然** |
| **F12** | **`GEMINI_MODEL` 环境变量无效——它根本没被读**（v0.57 bundle 里 `env.GEMINI_MODEL` 零命中）。真正的键是 `settings.json` 的 **`model.name`**，已实测生效 | §4.3 写 `GEMINI_MODEL=gemini-2.5-flash` | 🔴 **改写 `settings.json` 的 `model.name`** —— F10 的认证键在同一份文件里，零额外成本 |
| **F9** | **「装没装」不能用配置目录判断** —— 实测装 **ChatGPT 桌面应用**会创建一个内容丰富的 `~/.codex/config.toml`（`[marketplaces.openai-bundled]` / `[plugins…]` / `[mcp_servers.node_repl]`），但 `codex` **在 PowerShell 与 cmd 里都不是命令**。| §4.2 闸 2 写的是「配置目录存在 **或** 可执行文件在 PATH」 | **判据改为可执行文件**。OR 判据下脚本会认定 Codex 已装、写配置、并让用户敲一个**不存在的命令**。ChatGPT 桌面版装机量很大，不是边缘情况 |
| **F8** | **Windows 的持久化不能照 POSIX 镜像** —— `$PROFILE` + `env.ps1` 在默认的 `Restricted` execution policy 下不加载，且**每次开终端报红**。改用 `[Environment]::SetEnvironmentVariable(…,'User')`（写注册表，与 policy 无关） | D2 的语法表里 Windows 一行是「`. $HOME\.deeprouter\env.ps1`」 | **D2 的 Windows 机制整条改写**，连带 §4.3 清单与 §4.6 卸载；并推翻了「fish 是唯一能弄坏终端的路径」这句 |
| **F21** | **脚本输出用英文** —— 首要理由是产品的:**有不懂中文的用户**(@sam,2026-08-28)。技术约束刚好同向:PS 5.1 按**系统 ANSI 码页**读 `.ps1`,实测同一行中文 `irm \| iex` ✅ 正常、存成无 BOM 的 `.ps1` ❌ 乱码、带**真** BOM ✅ 正常 —— 而两步写法(§5.4)恰恰要存文件。⚠️ 首轮测量把 BOM 那组做错了(`UTF8Encoding.GetBytes()` **不输出 preamble**,BOM 要单独写),改对后结论才反转;`chcp 936`(中文 Windows 默认)下的**控制台渲染测不准**,输出被工具捕获后重新解码 | §2.2 的样例输出与 §6 的失败态文案都是中文 | **两份脚本输出一律英文**;网关返回的中文错误**翻译后输出、数字原样保留** —— §6 要的本来就是「剩余 $X / 需要 $Y」这两个数字,不是那句中文。§2.2 的中文样例降级为**示意**,不是逐字规格。🔴 **这不是「等编码问题解决就改回中文」的临时妥协** —— 用户群里有人不读中文,所以即使 BOM 那条路走通了,结论也一样 |
| **F22** | 🔴 **Windows 写环境变量很慢,而且慢得没有提示** —— `[Environment]::SetEnvironmentVariable(…,'User')` 会向桌面上所有顶层窗口广播 `WM_SETTINGCHANGE` 并**阻塞等待回应**,实测**每个变量约 7.1 秒**。四个工具全配 = 7 个变量 ≈ **50 秒**,而这 50 秒正好落在「Writing configuration…」之后、**没有任何输出**。用户会以为卡死并按 Ctrl+C。⚠️ 试过「直接写注册表 + 最后广播一次」把它压到 ~7 秒(实测 7 次写入仅 32ms、REG_SZ 正确、`%` 不展开),但该写法出现**间歇性写不进去且不报错**,原因未查清 —— 已**回退**,V5–V8 验的是 `SetEnvironmentVariable`,不是直接写注册表 | 全文未提安装耗时 | **保留已验证的写法,改为写之前先告知耗时**。压缩到一次广播是一条有价值的优化,但要先解释清楚那个间歇性失败,**不能在「工具能不能用」这一环上用没验透的机制** |
| **F23** | 🔴 **F7 的修法本身还不够** —— 改用 `Invoke-WebRequest` 后,`$_.Exception.Response.GetResponseStream()` **已经被读完了**:流报告 `CanRead=True` / `CanSeek=True`,然后**读出 0 字节**(因为位置在末尾)。正文其实在 `$_.ErrorDetails.Message` 里。**而这个失败是隐形的** —— message 为空 → 分类落到「未知 4xx」→ 每一次预扣费失败都被静默当成「令牌无权访问该模型」,**§6 里所有基于 message 的规则在 Windows 上全部失效**,用户永远看不到「剩余 $X / 需要 $Y」 | F7 只说了「改用 `Invoke-WebRequest` 手动读响应流」 | **先读 `ErrorDetails.Message`,流只作兜底**(兜底时先 `Position = 0` 并显式用 UTF8 读)。顺带把「网关没声明 charset」也一起防住了 —— 实测两种响应现在都能正确抽出两个数字 |
| **F24** | 🔴 **Codex 的 profile 文件里,键必须在顶层** —— `codex --help` 原文:`--profile <name>` 是「Layer `$CODEX_HOME/<name>.config.toml` on top of the base user config」。**层进来的是整个文件**,所以 `model` / `model_provider` 要写在顶层;包一层 `[profiles.deeprouter]` 会把它们埋进一个没人激活的嵌套 profile。**失败是静默的**:Codex 正常加载该文件、正常启动,然后 header 显示 `provider: openai`、请求打向 `wss://api.openai.com/v1/responses`。实测 codex-cli 0.149.1(2026-08-28,真机):**只差那一行 header**,`provider` 就从 `deeprouter` 变成 `openai` | §4.3 的内容块本来就是顶层的(没错),V10 也验过 —— **错的是实现**:一层多余的 `[profiles.x]` | 🔴 **两种情况写的内容逐字相同,只有文件名不同**(§4.3 原话「内容都是同一段」)。已加测试钉住:profile 文件里不得出现 `[profiles.`,且两个分支产出必须 `Equal`。📌 **单测看不出来** —— 文件内容在任何单测视角下都是对的,只有真装了 Codex 的机器能发现 |
| **F25** | 🔴 **OpenCode 只加 provider 不够，还必须设顶层 `model` 默认** —— 没有它，OpenCode 启动仍用自家目录挑的模型（实机 2026-08-28：Nano Banana Pro，张口要 Google key），用户读作「没配上」。DeepRouter 其实已在模型列表里，只是没被选中 | §4.3 的 JSON 块只有 `provider` 一段；V9 验的是 `opencode models` **列出**模型，没验**启动时选中**哪个 | 两份脚本顶层补 `"model": "deeprouter/<选中的模型>"`，用户自设的默认**有意覆盖**（备份 + 可卸载还原）。测试钉住：全新装与已有配置两条路径，顶层 `model` 都必须等于 `deeprouter/<模型>`。📌 与 F24 同款：**文件在单测视角下全对，只有真启动工具才暴露** |
| **F26** | 🔴 **真网关的钱符是全角＄，且随显示设置可变（¥/¤/自定义）** —— `logger.FormatQuota` 按网关的计价显示设置出符号，默认 USD 分支用全角＄（U+FF04）。两份脚本的金额提取只认半角 `$`，于是「余额不足」分类命中但两个金额永远取不出，§6 的钱数文案在真网关上从不出现；消息尾的 `(request id: …)` 还带数字，取数必须先剪尾 | 假网关当初写的是半角 `$`，测试因此全绿 —— **F23 同族的假上游不忠实** | 两份脚本改为**剪掉 request id 尾后取最后两个数字、不认任何货币符**；假网关消息改成与真网关逐字节同形（含全角＄与 request id 尾）。发现方式：L163 验收在真网关上人为造出预扣费失败 —— 行为测试报不了的那类。⚠️ 已知边界：英文文案里的 `$` 前缀在网关配成 CNY 显示时会错标币种（生产 USD 计价，暂接受） |
| **F27** | 🔴 **死令牌的应答必须是会说话的脚本 + HTTP 200** —— 旧实现是「全注释正文 + 404」：注释经 `sh` 零输出，而 `curl -f` 遇非 2xx 直接丢弃正文 —— 双重保证用户什么都看不到，实测 exit 0 彻底静默（§6 首行明令禁止） | 当初的注释写法是为了「安全：不执行任何东西」，测试还把它钉成了铁律 | 改为按 UA 分平台的 `echo`/`Write-Host` 脚本 + `exit 1`，状态 200；测试改为**真跑 sh** 断言有声音且非零退出。L210 行 6 实造抛出 |
| **F28** | 🔴 **最终写入不检查 = 假成功** —— 只读 opencode.json 下：裸 Permission denied 漏出后照样 `[ ok ]` + `Done. 1 tool(s) configured`；验证阶段打的是网关，探不到文件没写进去 | `cat > file` 的返回值没人看；新建路径有检查、覆写路径没有（opencode/gemini 两处），env.sh 与清单同病 | 四处全部改检查：失败即 `[fail] cannot write <file>` + 权限提示，不记清单不报成功；ps1 本就全在 try/catch 里（.NET 异常是终止性）无需改。回归测试用 chmod 0444（Windows 上是只读属性，两平台都真拦写）。L210 行 5 真 Linux 容器抛出 |
| **F29** | 🔴 **断网时把用户往密钥问题带** —— 文案是「检查你的密钥是否允许…」。双层缺陷：① 探测循环没有 network 分支；② posix 侧 `dr_http` 里 curl 失败时 `-w` 已印 000、`|| printf 000` 又追加一个 → 返回 `000000`，分类器的 `000` 分支**从来就是死代码** | 分类表里有 network 类，但没有任何路径消费它；PS 侧自己的异常路径反而是对的 | 两脚本探测循环加 network 分支（断线即停止探测）+ 专属总结「Cannot reach DeepRouter…check your network」；`dr_http` 归一化不再追加。L210 行 11 `--network none` 容器抛出，新测试打向拒连端口钉住 |
| **F30** | ⚪ **Codex 与 Claude Code 都不认识 `deeprouter-auto`**，各自打一条警告（Codex：`Model metadata for … not found`；Claude Code 2.1.250：`…is not a model this version recognizes, so auto-compact will keep this session within 200k tokens`）。两者都**不影响对话**（均已实机跑通），影响的是它们对上下文窗口的假设：假设偏大时，长会话会在中途撞上上游拒绝 | 两个工具都靠内置的「模型名 → 窗口」表，而 `deeprouter-auto` 是虚拟名，不在任何人的表里；且它每次请求可能路由到不同模型，**本就不存在单一真值** | 🔴 **已拍板：不做（@sam 2026-08-28，原话「不值当」）**。出路是有的 —— Claude Code 接 `CLAUDE_CODE_MAX_CONTEXT_TOKENS`、Codex 接 `model_context_window`（两个键均已实测存在且被解析），但**没有诚实的数字来源**：实测 `/v1/models` 只有 `id/object/created/owned_by/supported_endpoint_types`，smart-router catalog 有价格与能力标签但**无上下文窗口**。在脚本里硬编一张模型表，正是导致这个问题的那类错误。若将来重提，应作为一个统一议题：让 catalog 暴露 `context_window`，一次解决两个工具 |

> F2/F3 合起来印证了 `key-setup-guide-prd.md` §6.3 早就写下的硬约束：**「Base URL / model 只能来自后端 API 注入，前端不得硬编码」**。那条规则原本是为「显示了 dev 端口 17231」那次事故写的；现在它有了第二个、更严重的理由。

#### 两套部署 —— 正式的是 `deeprouter.co`

存在**第二套独立部署 `deep-router.com`**：同一份代码、独立数据库、**密钥互不通用**
（一套的 key 打另一套 401，已验）。完整对照表与定位归 **`deeprouter-ai/deploy/README.md`
§"There is a second deployment"** 管，本 PRD 只留对规格的影响：

🔴 **§4.1「Base URL 取该实例自己的 `server_address`」是必须，不是防御性设计** ——
写错地址的用户一路 401。本条结论曾被答错过一次（详见 deploy/README.md 与 LOG），
连人都会答错，脚本更不能把答案写死。

#### 在 `.co` 上的重测（2026-08-26，Cherry Studio + curl）

POC 全部做在 `.com` 上。**工具侧机制**（F1 / F8 / F9 / F10–F15）与网关无关、不受影响；
**部署侧事实**必须重测，结果如下（旧密钥属于 `.com`，在 `api.deeprouter.co` 上 401，已另开账号）：

| # | 重测项 | 结果 |
|---|---|---|
| **R1** | 模型目录 | 🔴 **两套不一样**。按 `.com` 量到的 `claude-opus-4-8` 写死，在正式部署上直接 `model_not_found` —— §4.3「先探测再写」的实证 |
| **R2** | 计费链路 | ✅ **通**。`gpt-4o-mini` 发 `hi` 正常回话，密钥→鉴权→选模型→预扣费→上游→计费全程跑通 |
| **R3** | 预扣费门槛 | 🔴 同一账号同一句 `hi`：Opus 4.7 thinking 要预扣 **$0.816**（余额 $0.04 → 403）；`gpt-4o-mini` **直接通过**。预扣额 = 单价 × `max_tokens`，**与实际用量无关** → **F16** |
| **R4** | `deeprouter-auto` | 🔴 曾 **100% 失败**：三个 prompt 选出**两个不同模型**——smart-router 是活的，挡住的是**令牌白名单**。✅ **已修复**（ADR-0007，`e13548a8`，本地 stash 前后对照验证：403→200；**待合并部署，`.co` 上今天仍是坏的**） |

根因与修法在 **`docs/adr/0007-auto-model-token-whitelist.md`**（一句话版：目录按**租户**算、
权限按**令牌**算，smart-router 的选择合法但令牌开不了；修复让解析结果在白名单**内**重选）。

**对规格的两个影响（修复后更新）：**

1. **§4.3 探测第 1 步（先试 `deeprouter-auto`）仍然必须** —— 修复在分支里，哪些部署带着它、
   目录长什么样，都随部署变；探测是唯一不赌部署状态的做法。
2. ✅ **P1 的未决约束已解除**：修复后**带白名单的令牌也能用 `deeprouter-auto`**（路由在白名单内进行），
   P1 不必再在「不带白名单」和「等修复」之间二选一。

> 修复卡「Fix: deeprouter-auto resolves to models the token may not use, 403 on every request」在 **eval**。
> ⚠️ 与 Gemini 那条**是两个独立 bug**（一个在解析前取不到消息，一个在解析后令牌用不了），修好任一条另一条依旧成立
> —— **且必须先修本条**：Gemini 那条今天正替它挡着子弹，单独修 Gemini 会让线上从「静默用错模型」变成 403。

#### 四种协议 × 具体模型名（2026-08-26，生产 `.co`，curl）

R4 测的是 `deeprouter-auto`；脚本实际写的是**具体模型名**，所以又跑了一遍。
🔴 **§4.4「四种协议各验一次」由此第一次真正在正式部署上跑通。**

| 协议 | 模型 | 结果 |
|---|---|---|
| OpenAI `/v1/chat/completions` | `gpt-4o-mini` | ✅ 200 |
| Anthropic `/v1/messages` | `claude-sonnet-4-6` | ✅ 200 |
| Responses `/v1/responses` | `gpt-4o-mini` | ✅ 200 |
| Gemini `/v1beta/…` | `gpt-4o-mini` | ✅ **200** ← 协议本身没问题 |
| Gemini `/v1beta/…` | `gemini-3.5-flash` / `gemini-2.5-flash` | 🔴 403 —— **该部署 Gemini 渠道为 0** |

#### 🔴 两个模型接口互不包含 —— 这是 §4.3 探测流程的地基

| | 数量 |
|---|---|
| `/v1/models`（**令牌**口径） | 29 |
| `/api/pricing`（**租户**口径，`TryUserAuth()`，无需登录） | 77（claude 26 · gpt 25 · doubao 16 · eleven 4 · chatgpt 2 · deepseek 2 · glm/omni 各 1 · **gemini 0**） |
| **交集** | **25** |
| 🔴 **只在 `/v1/models`：能用，却任何地方都查不到价格** | **4** —— `gpt-4o-mini` · `gpt-4o` · `claude-sonnet-4-6` · `claude-opus-4-7` |
| 只在 `/api/pricing`：有价格，令牌却用不了 | 52 |

- 🔴 那 4 个「能用但无价格」的**恰好包含实测跑通 200 的两个** → **F16 的「按价格升序试」按原文不可实现**（F19）。
- ✅ 同时坐实令牌白名单那个 bug：smart-router 挑中的 `claude-haiku-4-5-20251001` / `deepseek-v4-flash` **在 77 里、不在 29 里**。
- 📌 **两个口径都不完整。** 只看 `/v1/models` 就会把「模型不存在」和「这张令牌不能用」混为一谈。

#### 🔴 Gemini CLI：三条路，三个不同的堵点

| Gemini CLI 配的模型 | 结果 | 堵点 |
|---|---|---|
| `gemini-*` | 403 | 该部署确无 Gemini 渠道 |
| **`gpt-4o-mini`** | 🔴 **400** `Unrecognized request argument supplied: top_k` | 网关把 Gemini 的 `top_k` 原样塞进 OpenAI 请求 |
| **`claude-sonnet-4-6`** | 🔴 **500** `convert_request_failed` / `not implemented` | 网关没有 Gemini→Claude 转换 |

🔴 **决定性对照：同一端点用 curl 打 `gpt-4o-mini` 是 200 —— 差别只在 CLI 会发 `top_k`。**
Gemini→OpenAI 本身是通的，**坏在一个参数上**。

- ✅ **模型名是能设的**（F12 已查清）：`env.GEMINI_MODEL` 在 v0.57 的包里**零命中**，
  真键是 `settings.json` 的 **`model.name`**，已实测生效 —— 而脚本本来就要写那个文件，**零额外成本**。
- ✅ **D1 不变，Gemini CLI 留在 Phase 1。** 要修的是网关 → 卡「Fix: Gemini-protocol requests fail
  on every model, top_k leak and no Claude converter」，其 `top_k` 那半约 2h。

> 📌 教训（详见 LOG）：**标着「未查清」的发现不能作为砍范围的依据** —— D1 曾因此误砍过 Gemini CLI，已撤回。

### ⚠️ 另外四条工程/文案发现

| # | 发现 |
|---|---|
| F4 | **`docs/integrations/claude-code.md` 里「按 Esc 跳过 Anthropic 登录」在 v2.1.177 上无效**——首启先是主题选择，再是三选一的登录方式，Esc 无反应。**引导流程随版本变，验收必须实测**（§7 已有此要求，现有实证） |
| F5 | **Claude Code 把网关的 403 前缀上了错的建议**：`Please run /login · API Error: 403 预扣费额度失败，用户剩余额度：$0.039042…`。网关原文**确实也显示了**（比最初以为的好），但用户先看到的是“去登录”，而真实原因是余额不足。**这正是脚本必须自己发验证请求并翻译错误的理由**（§6）。已在 V11 里一字不差重现 |
| F6 | 三处编码陷阱，方向互相矛盾：**`.ps1` 必须纯 ASCII**（PS 5.1 按系统 ANSI 读，非 ASCII 注释会冲掉引号导致解析失败）· **写出的 JSON 必须无 BOM** · **HTTP 请求体必须显式转 UTF-8 字节**（否则中文变 `?`，且**不报错**） |
| F7 | **`Invoke-RestMethod` 会吞掉 HTTP 错误正文**，只剩「(401) 未经授权」。§6 的失败态映射依赖网关返回的 message，必须改用 `Invoke-WebRequest` 手动读响应流 |

### 尚未验证

三轮 POC 各自只隔离一件事 —— 这是**刻意的**：一次验一个变量，失败时才分得清是机制不通还是自己的代码有 bug。

**验过的只是 Windows 上「环境变量」这一条线**（F1 工具确实读环境变量 · V5 注册表到达新终端 · V8 按名删除干净），而且**从没有一次是串起来跑的**。除此之外，本 PRD 的绝大部分仍是纸面推理。按风险从高到低：

| # | 空白 | 为什么要紧 |
|---|---|---|
| **U1** | ✅ **§4.2.1 的检测信号已核实存在**（2026-08-26）——`~/.claude/.credentials.json` 是个普通文件（566b），**存在性判断即可**；Windows Credential Manager 里没有对应条目，所以不必碰系统钥匙串。**剩下的是各工具的信号还没一一核实**（U6） | 原本是全 PRD 风险最高的空白：这是唯一一个「脚本完全正常工作、却让用户损失金钱」的路径。现在机制可行，风险从「做不做得到」降为「做不做得全」 |
| **U2** | ✅ **已跑通**（`.com`，V11）+ ✅ **已在 `.co` 上重跑**（2026-08-26）—— Claude Code 用进程级环境变量指向 `api.deeprouter.co`，**请求到达网关**（403 预扣费 + request id），F15「claude.ai connectors are disabled」同样复现。🔴 **重跑的价值不在部署，在「真客户端 ≠ curl」** ——正是这一跑测出了 **F20**（Claude Code 实发要预扣 $0.16，而 curl 最小请求 200）。📌 持久化那半（注册表 → 新终端）**与部署无关，无需重测** |
| U3 | ⬇️ **降级** · 默认 `Restricted` execution policy 下的行为（F8） | **改走注册表后，execution policy 对所选机制已不再相关**（`SetEnvironmentVariable` 是 .NET 调用，`irm \| iex` 也不受限）。从「设计风险」降为「回归检查」——只需确认脚本里没有别的地方依赖执行 `.ps1` 文件。开发机通常已改成 `RemoteSigned`，仍然测不出来 |
| **U4** | ✅ **已实测点通**（2026-08-26，见 §4.7 B1）—— 深链唤起、确认框、基础 URL 逐字正确、模型自动拉到 2 个 | 原本是本 PRD 证据最弱的一处，且 §10 的排序建议架在它上面。**现在它是证据最强的一处** —— 唯一一条被端到端点通的完整用户路径。顺带推翻了本 PRD 自己的一个预判（以为用户还得手输模型 ID），并暴露一个改不掉的问题：**品牌显示为「New API」** |
| **U5** | **后端令牌接口**（`GET /i/{token}`、单次使用、15 分钟过期、携带勾选清单） | 整个后端半边零验证。POC 全程是把密钥手敲进脚本的 |
| U6 | ✅ **已关闭** —— 四个工具**全部**实测到网关 | 结果分化得很彻底：**Gemini 3/3 全错**（F10–F12）、**Codex 两错一对**（F13/F14 错，Q7 机制对）、**OpenCode 逐字全对**。共同点很清楚：**靠官方文档推导出来的都错了，靠查实际配置路径得出来的都对** |
| U7 | macOS / Linux **一次都没跑过**（⬇️ 大部分可用 WSL 覆盖，见下） | POSIX 那半的 `env.sh` + 一行引用完全没验，fish 更没有 —— 而 fish 是 §7 里点名的破坏性风险 |
| U8 | **配置文件的合并保护与备份还原**（`original_backup`） | V8 验的是**环境变量**按名删除，不是文件还原。这两件事在 Windows 上机制完全不同 |
| U9 | ❌ **基本作废** · 全局 `~/.claude/settings.json` 的行为是否与项目级不同 | **F8 之后 Claude Code 根本不再写 `settings.json`**（改走环境变量），这个问题失去了对象。留在表里只是记录它曾经是个未知数 |

> 📌 **U3 / U9 是 F8 换机制的副产品** —— 换机制不只是换实现，它顺带退掉了两个未知数。
> 反过来说：**一个靠「等实测再看」兜底的设计比看起来更贵**，每个未知数都要占一次真机验证。

🔴 **U2 / U3 有个共同点:在开发者自己的日常机器上做不到或不该做**（一个烧掉他的订阅，
一个被他改过的 policy 掩盖）。**这类「验不了的东西」必须写进验收并写明前置条件**，
否则会一路滑到上线 —— 因为每一次「我这边跑通了」都是真话。§7 已单列。

#### 还剩什么没验

**U1 / U4 / U6 已关闭**（开工前该做的三条）。剩下的 **U5 · U7 · U8** 都是「被验的东西还不存在」
（真脚本 / 真后端 / 真合并逻辑），**不阻塞开工**。

##### U7 的实际做法：不要等一台 Mac

本 PRD 承诺双平台，而 `TEAM.md` **没记录谁用什么操作系统** —— 但这不该阻塞开工，
POSIX 那半的风险绝大部分与 macOS 无关：

| 要验的 | 在哪验 |
|---|---|
| `env.sh` + 一行引用、幂等、卸载删行 · **fish 语法**（§7 点名的破坏性风险）· bash/zsh | **WSL**（`wsl --install`，免费） |
| 四个工具的配置路径 | `~/.codex` / `~/.gemini` / `~/.config/opencode` —— **XDG 风格，mac 与 Linux 一致** |
| `curl \| sh` · `~/Library/Application Support/…` | 两边相同 / **这四个工具都不用它** |
| ⚠️ **「macOS 默认 shell 就是 zsh」这一点** | 只能在真 Mac 上确认（语法本身 WSL 可覆盖） |

📌 **先用 WSL 把 POSIX 那半验掉。** 真要补，GitHub Actions 的 **macOS runner 免费**，
可跑非交互那部分；只有「开新终端、`claude` 直接进输入框」需要真人真机。

> ⚠️ **「谁用什么系统」值得记进 `TEAM.md`** —— 这次是 macOS，下次可能是「谁能测 Windows 默认 policy」、
> 「谁装了 Cherry Studio」。**每次现问一遍，就是每次都会漏一次。**
