package skill

import "embed"

// PackageAssets holds bundled scripts and reference files for each skill slug.
// New skill packages go under packages/<slug>/scripts/ and packages/<slug>/reference/.
//
//go:embed packages/academic-polish packages/code-review-ds packages/smart-translate packages/prompt-optimizer packages/youtube-analyzer packages/agent-memory packages/self-improving-agent packages/self-reflection
var PackageAssets embed.FS
