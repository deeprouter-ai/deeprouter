# Prompt Engineering Techniques — Quick Reference

## Core Principles

### 1. Clarity over Cleverness
Every word should earn its place. If removing a phrase doesn't change the output, remove it.

**Weak:** "Please could you kindly help me with writing something that is kind of like a blog post about AI"
**Strong:** "Write a 600-word blog post for software developers explaining transformer attention mechanisms. Tone: technical but accessible. No analogies involving water."

### 2. Role Assignment
Tell the model WHO it is, not just WHAT to do. Roles activate relevant training knowledge.

```
You are a senior TypeScript engineer at a fintech company. You write defensive,
well-typed code with explicit error handling. You do not use `any`.
```

### 3. Output Format Specification
Models default to prose. Override explicitly.

```
Output format:
- Numbered list of issues
- Each issue: [SEVERITY] File:Line — Description — Fix
- End with: Total: X issues found
```

---

## Structural Techniques

### XML Tags (best for system prompts)
```xml
<task>Summarize the following legal document</task>
<constraints>
  - Maximum 200 words
  - Use plain English (no legal jargon)
  - Highlight any liability clauses
</constraints>
<format>Markdown with headers for each section</format>
```

### Chain-of-Thought
Add to reasoning tasks:
```
Think step by step before giving your final answer.
First identify..., then consider..., finally conclude...
```

### Prefilling (Claude-specific)
Start the model's response to force format:
```
User: Classify this email as spam/not-spam.
Assistant: Classification: 
```

### Few-Shot Examples
Use 2-3 examples for format-sensitive tasks:
```
Input: "The product broke after 2 days" → Sentiment: NEGATIVE, Topic: QUALITY
Input: "Arrived early, great packaging" → Sentiment: POSITIVE, Topic: DELIVERY
Input: "{{user_input}}" → 
```

---

## Constraint Patterns

### Negative Constraints (what NOT to do)
```
Do not:
- Use bullet points (write in paragraphs)
- Mention competitor products by name
- Include pricing information
- Exceed 100 words
```

### Scope Constraints
```
Only answer questions about Python 3.10+. If asked about other languages or
versions, say "I can only help with Python 3.10+ in this context."
```

### Quality Gates
```
Before responding, verify:
1. The code compiles (mentally trace it)
2. Edge cases are handled: empty input, null, max values
3. The explanation matches the code
```

---

## Anti-Patterns to Avoid

| Anti-Pattern | Problem | Fix |
|---|---|---|
| "Write a good essay" | "Good" is undefined | "Write an essay scoring 8/10 on clarity, with 3 supporting arguments" |
| "Be creative" | Too open-ended | "Generate 5 unconventional ideas, each must be technically feasible within 6 months" |
| "As an AI language model..." | Wastes context | Remove entirely |
| Over-specified personas | Creates contradictions | Keep role to 2-3 key traits |
| Hallucination bait | "What did X say about Y?" | "If X has written about Y, summarize; otherwise say you don't know" |
| Nested instructions | Model loses track | Flatten or use numbered list |

---

## Model-Specific Notes

### DeepSeek / DeepRouter Models
- Responds well to explicit reasoning instructions
- Use `<think>` tags for complex reasoning tasks
- Temperature 0.1-0.3 for factual tasks, 0.7-0.9 for creative

### Reducing Hallucinations
```
Only use information from the provided context. If the answer is not in the
context, say "I don't have enough information to answer this." Do not infer
or extrapolate beyond what is explicitly stated.
```

---

## Evaluation Checklist

Before finalizing a prompt, ask:
- [ ] Is the task clear to someone who knows nothing about context?
- [ ] Is the output format specified?
- [ ] Are success criteria defined?
- [ ] Are the most common failure modes addressed with constraints?
- [ ] Is the persona (if any) consistent throughout?
- [ ] Would removing any sentence change the output? (If not, remove it)
