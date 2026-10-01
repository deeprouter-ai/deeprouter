// Package packageassets holds the static content that gets bundled into
// every skill package: the shared README template. Identical across all
// skills — only the slug gets substituted in at packaging time.
package packageassets

import _ "embed"

//go:embed runtime_readme_template.md
var ReadmeTemplate string
