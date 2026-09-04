# DeepRouter Skill Runtime / DeepRouter 技能运行环境

## 1. Prerequisite / 前提条件

This skill requires a DeepRouter API Key to run. If you don't have one yet,
register and top up your account at https://deeprouter.co.

运行这个技能需要一个 DeepRouter API Key。如果还没有，请先在
https://deeprouter.co 注册并充值。

## 2. Set the API key / 设置 API Key

**macOS / Linux:**

```bash
export DEEPROUTER_API_KEY=sk-dr-xxx
```

**Windows:** open System Properties → Environment Variables, and add a new
user variable named `DEEPROUTER_API_KEY` with your key as the value.

**Windows：** 打开"系统属性 → 环境变量"，新增一个名为 `DEEPROUTER_API_KEY`
的用户变量，值填你的 key。

## 3. Install / 安装

Extract this package to:

```
.claude/skills/{slug}/
```

解压这个包到：

```
.claude/skills/{slug}/
```

## 4. Use it in Claude Code / 在 Claude Code 中使用

Once installed, Claude Code automatically picks up this skill from
`SKILL.md` and invokes it when relevant to your request — no extra setup
needed beyond steps 1-3 above.

安装完成后，Claude Code 会根据 `SKILL.md` 自动识别这个技能，在合适的场景
下调用它——除了上面 1-3 步，不需要额外配置。
