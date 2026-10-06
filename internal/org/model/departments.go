package model

// Languages a preset department has a name in. They are the three the gateway
// answers in (i18n.LangEn / LangZhCN / LangZhTW), spelled out here because this
// package stays free of platform imports.
const (
	langEn   = "en"
	langZhCN = "zh-CN"
	langZhTW = "zh-TW"
)

// PresetDepartment is one department every new organization starts with.
type PresetDepartment struct {
	Key       string            // stored in departments.preset_key
	IsDefault bool              // the catch-all department, which cannot be deleted
	Names     map[string]string // the name it is created with, per language
}

// PresetDepartments is the starter org chart of PRD §5 (decision D23): the
// default department plus five common business units. The list is short on
// purpose, and IT / HR / finance / audit are missing on purpose — those are
// jobs, expressed through roles, not places people sit.
//
// It only shapes organizations created from now on: the rows are copied in
// once, after which the organization renames and deletes them freely.
var PresetDepartments = []PresetDepartment{
	{Key: "general", IsDefault: true, Names: map[string]string{langEn: "General", langZhCN: "综合", langZhTW: "綜合"}},
	{Key: "engineering", Names: map[string]string{langEn: "Engineering", langZhCN: "技术", langZhTW: "技術"}},
	{Key: "product", Names: map[string]string{langEn: "Product", langZhCN: "产品", langZhTW: "產品"}},
	{Key: "marketing", Names: map[string]string{langEn: "Marketing", langZhCN: "市场", langZhTW: "市場"}},
	{Key: "sales", Names: map[string]string{langEn: "Sales", langZhCN: "销售", langZhTW: "銷售"}},
	{Key: "support", Names: map[string]string{langEn: "Customer Support", langZhCN: "客服", langZhTW: "客服"}},
}

// Name returns the preset's name in lang, or in English for a language it has
// no name in — the same fallback the gateway's own messages use.
func (p PresetDepartment) Name(lang string) string {
	if name, ok := p.Names[lang]; ok {
		return name
	}
	return p.Names[langEn]
}
