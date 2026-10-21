# Service build and production execution boundary

Service dependencies, validation, compilation and image assembly/push run on the existing independent Mac CI. Desktop installers and desktop platform acceptance retain GitHub-hosted macOS/Windows jobs. Production only pulls completed images, applies runtime configuration, checks health and rolls back. Never run source builds on production or forward Docker/BuildKit to production.

Before merging/running these workflows, bind repository variable `SERVICE_CI_RUNNER_LABELS` to the JSON label array of an existing, verified Mac CI runner available to this repository (including `self-hosted` and its `*-mac-studio` role). There is deliberately no hosted Ubuntu, generic self-hosted, or production fallback. This source-only task does not register/start listeners or verify a new binding.

Preserve existing tests, job dependencies, environments, approval and exact-commit deployment gates. Do not deploy a source branch whose required checks have not passed. This change authorizes no deployment or desktop release.

SERVICE_CI_RUNNER_LABELS must select the existing Linux ARM64 VM role on Mac CI. SERVICE_CI_MACOS_RUNNER_LABELS must separately select an existing native macOS ARM64 role for the macOS server binary. Electron desktop packaging stays on GitHub-hosted platform runners. Do not bind either variable to a generic runner pool.
