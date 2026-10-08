package model

// PolicyTemplate is a ready-made answer to "which models may this key call"
// (PRD §5, decision D14): a set of the purposes personal keys are already
// created by. Applying one resolves its purposes to models and writes them into
// the key's whitelist; the template's key stays on the key
// (tokens.policy_template), so it can be applied again when the catalogue grows.
type PolicyTemplate struct {
	Key      string   `json:"key"`
	Purposes []string `json:"purposes"`
}

// PolicyTemplates is the one and only definition of the policy templates. v1
// has the two the platform offers; organizations do not build their own (PRD
// §8). The purposes are the ids of setting/alias_setting and
// internal/keypurpose, which is what they are resolved through.
var PolicyTemplates = []PolicyTemplate{
	{Key: "creative", Purposes: []string{"image", "video", "chat"}},
	{Key: "coding", Purposes: []string{"coding"}},
}

// FindPolicyTemplate looks a policy template up by its key.
func FindPolicyTemplate(key string) (PolicyTemplate, bool) {
	for _, template := range PolicyTemplates {
		if template.Key == key {
			return template, true
		}
	}
	return PolicyTemplate{}, false
}
