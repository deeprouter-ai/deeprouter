# Self-Reflection Guide — Getting the Most from AI Retrospectives

## When to Reflect

| Trigger | Style | Frequency |
|---|---|---|
| End of coding session | `--style dev` | Daily |
| After debugging a hard bug | `--style dev` | As needed |
| After AI agent completes a task | `--style agent` | Per task |
| After learning something new | `--style learning` | Per session |
| Weekly review | `--style dev` | Weekly |

---

## What to Include in Your Notes

Better input → better reflection. Include:

### For dev sessions
- What you set out to do
- What you actually did (might differ)
- Specific blockers you hit (error messages help)
- Decisions you made and why
- What surprised you
- How long things took vs. how long you expected

### For agent sessions
- The task given to the agent
- Tools the agent used
- Where it got stuck or made mistakes
- Whether it asked for clarification or just assumed
- Final output quality

### For learning sessions
- Topic / resource studied
- Key concepts encountered
- Examples that clicked (and ones that didn't)
- Questions that came up

---

## Reflection Styles Explained

### `--style dev` (default)
Blameless engineering retrospective. Focuses on:
- Technical wins and challenges
- Root cause analysis (not "what broke" but "why")
- Concrete, time-bound action items for tomorrow
- One key insight to remember

Best for: Daily coding sessions, sprint retrospectives, bug postmortems

### `--style agent`
AI performance review. Focuses on:
- Task completion rate
- Decision quality (did the agent make good judgment calls?)
- Tool usage efficiency
- Guideline compliance
- Scored performance (1-10 with justification)

Best for: Evaluating AI agent outputs, prompt debugging, agent workflow improvement

### `--style learning`
Knowledge consolidation. Focuses on:
- Concepts covered and understood
- Gaps and confusions that remain
- Connections to prior knowledge
- Practice exercises to reinforce
- 2-sentence summary (Feynman technique)

Best for: Courses, documentation reading, tutorials, code walkthroughs

---

## Sample Input Notes

### Dev session example
```
Worked on DeepRouter skill package download feature.

Goals:
- Add embed support for scripts/ and reference/ directories
- Update seed SQL with 3 new skills

What happened:
- Go embed kept failing with "no matching files found" — spent 45 min
  figuring out embed can't use ../ paths
- Fixed by moving embed to parent package level (packages.go)
- SQL seeds worked first try
- Tests pass

Surprises:
- Go's embed restriction is stricter than I expected
- The fix was elegant once understood

Time: 3 hours (expected 1.5)
```

### Agent session example
```
Task: Generate a 3-skill SQL seed file for DeepRouter marketplace.

Agent behavior:
- Started with INSERT without ON CONFLICT — I had to correct to idempotent version
- Correctly used deeprouter-ai/deeprouter repo target without being told
- Tried to create a PR despite "no PRs" instruction (had to redirect)
- Final SQL was correct and well-commented

Output: seed.sql with 3 entries, runs cleanly
```

---

## Using Reflections Effectively

### Save reflections in a structured way
```bash
python reflect.py notes.txt --style dev --out reflections/2026-06-23.md
```

### Build a weekly summary
```bash
cat reflections/2026-06-*.md | python reflect.py --stdin --style dev --out weekly.md
```

### Feed insights into agent memory
After a good reflection:
```bash
python ../agent-memory/scripts/memory.py learn \
  "Spent 3h debugging embed paths" \
  --context "Go development" \
  --outcome negative \
  --insight "Go embed cannot traverse upward (../); always place embed at same package level"
```

---

## Reflection Anti-Patterns

| Anti-pattern | Problem | Better approach |
|---|---|---|
| "Today went OK" | No signal | Name 3 specific things |
| Blaming tools/libraries | No learning | What was YOUR assumption that failed? |
| Only logging failures | Imbalanced | Also note what worked and why |
| Skipping after good days | Miss reinforcement | Good days have patterns worth repeating |
| Vague action items | Not actionable | "Try X next time" → "Tomorrow at 9am: do X for task Y" |
