// Package packageassets holds the static content that gets bundled into
// every skill package: the shared runner script and its README template.
// Both are identical across all skills — only the README gets a per-skill
// slug substituted in at packaging time.
package packageassets

import _ "embed"

//go:embed deeprouter_skill_runner.py
var RunnerScript string

//go:embed runtime_readme_template.md
var ReadmeTemplate string
