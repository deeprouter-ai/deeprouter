# Code Review Checklist

Use this alongside the AI review output. Manually verify CRITICAL and HIGH
findings. The AI may miss context-dependent issues or produce false positives.

---

## Security (Check Every PR)

### Injection
- [ ] All user input to SQL is parameterised / ORM-escaped (never string concat)
- [ ] Command execution uses argument lists, never shell=True / os.system with user data
- [ ] Template rendering escapes output by default; check for `|safe` / `Markup()` overrides
- [ ] XML/HTML parsers have external entity resolution disabled (XXE)
- [ ] SSRF: URL fetch targets are validated against an allowlist; internal IP ranges blocked

### Authentication & Session
- [ ] JWT signature algorithm is pinned server-side (never `alg: none`)
- [ ] Session IDs are cryptographically random (≥128 bits), rotated on login
- [ ] Password hashing uses bcrypt/argon2/scrypt with appropriate cost factor
- [ ] MFA bypass paths do not exist (fallback flows checked)
- [ ] Tokens stored only in httpOnly, Secure, SameSite=Strict cookies (not localStorage for sensitive tokens)

### Authorisation
- [ ] Every endpoint checks the caller's role/permission, not just authentication
- [ ] Object IDs in URLs/bodies are validated against the caller's ownership (IDOR)
- [ ] Admin endpoints are behind a separate middleware, not just a flag check

### Secrets
- [ ] No API keys, passwords, or tokens in source code or config files
- [ ] `.env` / `secrets.yaml` is in `.gitignore`
- [ ] Environment variable names documented in `.env.example` with placeholder values

### Cryptography
- [ ] MD5/SHA1 not used for passwords or security-sensitive hashes
- [ ] AES uses GCM or CBC with random IV (never ECB)
- [ ] TLS minimum version is 1.2; 1.0/1.1 disabled
- [ ] Random numbers from `secrets` module (Python) or `crypto/rand` (Go), not `random`/`math/rand`

### File & Path
- [ ] File uploads: type validated by content (magic bytes), not just extension
- [ ] Upload destinations are outside the web root
- [ ] Path parameters sanitised: `../` sequences resolved before comparison

---

## Correctness

### Error Handling
- [ ] All errors are checked — no naked `_` ignores on error returns (Go) / uncaught exceptions (Python/JS)
- [ ] HTTP error responses include a safe error message (no stack traces to client)
- [ ] Database errors do not leak schema details to the user

### Concurrency
- [ ] Shared state protected by mutex/lock or uses atomic operations
- [ ] No double-checked locking without memory barriers
- [ ] Goroutine/thread leaks: all launched goroutines have a cancel/stop path
- [ ] Channel operations cannot deadlock (no unbuffered channel send without a corresponding receive)

### Data Integrity
- [ ] Transactions used where multiple writes must be atomic
- [ ] TOCTOU (Time-of-Check-Time-of-Use): re-validate state inside the transaction, not before it
- [ ] Integer overflow: arithmetic on user-supplied values uses checked math or safe types

---

## Performance

### Database
- [ ] N+1 query pattern absent (use JOINs or batch loads)
- [ ] Query filters use indexed columns
- [ ] `SELECT *` replaced with explicit column lists
- [ ] Pagination in place for unbounded list endpoints

### Memory & CPU
- [ ] No O(n²) or worse algorithms in hot paths
- [ ] Large objects not copied when pointer/reference suffices
- [ ] File/stream processing uses iterators/streaming, not full read-into-memory for large files

---

## Maintainability

### Code Structure
- [ ] Functions are ≤50 lines and do one thing
- [ ] Public APIs have docstrings/comments explaining WHAT and WHY (not HOW)
- [ ] No magic numbers — constants are named
- [ ] No deep nesting (>3 levels) — extract into named functions

### Dependencies
- [ ] New dependencies are actively maintained (last release < 12 months)
- [ ] Dependency versions are pinned in lock file
- [ ] No transitive dependency conflicts

---

## Severity Definitions

| Severity | Definition | Typical Action |
|----------|------------|----------------|
| CRITICAL | Exploitable in production with no user interaction; data breach / RCE possible | Block merge; fix immediately |
| HIGH | Exploitable with minimal conditions; significant security or data loss risk | Fix before deploy |
| MEDIUM | Exploitable with specific conditions; moderate impact | Fix in next sprint |
| LOW | Minor risk; defence-in-depth improvement | Backlog |
| INFO | Best practice / style; no direct security risk | Optional |

---

## False Positive Guide

Common AI false positives to verify manually:

1. **"Hard-coded secret"** — check if the value is actually a placeholder (`"your-key-here"`) or test fixture
2. **"Missing error check"** — some Go/Rust patterns intentionally panic on programmer errors
3. **"SQL injection"** — ORM methods that look like string concat but are actually parameterised
4. **"Weak hash"** — MD5 is fine for non-security uses (checksums, cache keys)
5. **"Unhandled exception"** — framework-level exception handlers may exist higher in the stack
