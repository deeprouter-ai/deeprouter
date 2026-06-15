# internal/policy

Single-responsibility decision engine. Inputs: `(kids_mode, policy_profile)` from the tenant's `User` row. Output: a `Decision` struct of eight booleans (plus the normalised `Profile`) that downstream code (`relay/airbotix_policy.go`) consults to know which constraints to enforce.

**Status**: ✅ Implemented + unit-tested + wired via `relay/airbotix_policy.go`.

## What it does

Centralises the "what should we enforce for this tenant?" question in one place so the relay code doesn't repeat conditional branches. `kids_mode = true` overrides whatever the profile says and turns on all hard constraints.

```
DecisionFor(kidsMode=true, profile="anything") → all flags ON
DecisionFor(false, "kid-safe")                  → whitelist + ZDR + child-safe prompt + strip identifying + narrow input filter + strict output filter
DecisionFor(false, "adult")                     → adult prompt + narrow input filter
DecisionFor(false, "passthrough")               → no enforcement (default)
DecisionFor(false, "")                          → no enforcement (default)
DecisionFor(false, "<unknown>")                 → no enforcement (safe fallback)
```

## Public API

```go
type Profile string  // "passthrough" (default), "adult", "kid-safe"

type Decision struct {
    KidsMode                  bool
    Profile                   Profile
    EnforceModelWhitelist     bool
    EnforceZDR                bool
    InjectSystemPrompt        bool
    StripIdentifying          bool
    RunInputFilter            bool
    EnforceStrictOutputFilter bool
}

func DecisionFor(kidsMode bool, rawProfile string) Decision
```

| Field | Meaning |
|---|---|
| `KidsMode` | Tenant has `User.KidsMode=true`. |
| `Profile` | Normalised `Profile` value. |
| `EnforceModelWhitelist` | `kids.IsModelEligible(model)` must hold. |
| `EnforceZDR` | Force `store: false` for OpenAI-family providers. |
| `InjectSystemPrompt` | Prepend the profile-level system prompt (`policy.SystemPromptFor`, e.g. `kids.ChildSafeSystemPrompt()` for kid-safe). |
| `StripIdentifying` | Remove `user_id` / `family_id` / etc. before upstream send. |
| `RunInputFilter` | Check entry input text against the profile denylist (`policy.CheckInput`). |
| `EnforceStrictOutputFilter` | Response text must pass `kids.OutputFilter` (`internal/kids` §5) before reaching the client. |

The function is pure — no globals, no I/O, deterministic. Trivial to test, trivial to inline-reason about.

## Dependencies

- Nothing. No `internal/*` imports, no stdlib network/I/O calls.

## How it's wired

The decision is computed once per request (typically in a middleware or at the top of the relay handler), then read by enforcement code in `relay/airbotix_policy.go`:

```go
decision := policy.DecisionFor(user.KidsMode, user.PolicyProfile)
if decision.EnforceModelWhitelist && !kids.IsModelEligible(req.Model) {
    return errors.New("model_not_eligible_for_kids_mode")
}
if decision.StripIdentifying { kids.StripIdentifyingMetadata(reqBody) }
// ...
```

## Tests

`profile_test.go` covers:
- `kids_mode=true` forces all flags on (incl. `RunInputFilter` and `EnforceStrictOutputFilter`) regardless of profile
- `kid-safe` profile enforces all flags (incl. `RunInputFilter` and `EnforceStrictOutputFilter`) even when `kids_mode=false`
- `adult` profile injects the adult prompt and runs the narrow input filter, but does NOT enforce the strict output filter
- `passthrough` / empty / unknown → no enforcement (`EnforceStrictOutputFilter=false`)
- `Profile` value is normalised correctly

Run: `go test ./internal/policy/...`

## Adding a new profile

1. Add the new `Profile` constant.
2. Add a case in `DecisionFor` setting the appropriate flags.
3. Add at least three test cases (`kids_mode=true` + `kids_mode=false` + boundary).
4. Update `relay/airbotix_policy.go` if the new flags need new enforcement code.

If a new constraint cuts across multiple existing profiles, extend `Decision` with another boolean field — keep `Decision` as the single source of truth for "what to enforce."
