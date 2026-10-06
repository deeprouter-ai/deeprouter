package model

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// The starter org chart is PRD §5 (decision D23): the default department
// "综合" and five business units. This pins the list to the document.
func TestPresetDepartments_MatchThePRD(t *testing.T) {
	var keys, chinese []string
	for _, preset := range PresetDepartments {
		keys = append(keys, preset.Key)
		chinese = append(chinese, preset.Name(langZhCN))
	}
	require.Equal(t, []string{"general", "engineering", "product", "marketing", "sales", "support"}, keys)
	require.Equal(t, []string{"综合", "技术", "产品", "市场", "销售", "客服"}, chinese)
}

// Every organization needs exactly one department that cannot be deleted:
// members without a department, and everyone in a department that gets
// deleted, land there.
func TestPresetDepartments_HaveExactlyOneDefault(t *testing.T) {
	defaults := 0
	for _, preset := range PresetDepartments {
		if preset.IsDefault {
			defaults++
			require.Equal(t, "general", preset.Key)
		}
	}
	require.Equal(t, 1, defaults)
}

func TestPresetDepartments_FitTheirColumnsInEveryLanguage(t *testing.T) {
	seen := map[string]bool{}
	for _, preset := range PresetDepartments {
		require.NotEmpty(t, preset.Key)
		require.LessOrEqual(t, len(preset.Key), 32, "departments.preset_key is varchar(32)")
		require.False(t, seen[preset.Key], "duplicate preset key %s", preset.Key)
		seen[preset.Key] = true
		for _, lang := range []string{langEn, langZhCN, langZhTW} {
			name := preset.Name(lang)
			require.NotEmpty(t, name, "%s has no %s name", preset.Key, lang)
			require.LessOrEqual(t, utf8.RuneCountInString(name), 64, "departments.name is varchar(64)")
		}
	}
}

func TestPresetDepartment_NameFallsBackToEnglish(t *testing.T) {
	general := PresetDepartments[0]
	require.Equal(t, "General", general.Name("en"))
	require.Equal(t, "綜合", general.Name("zh-TW"))
	require.Equal(t, "General", general.Name("fr"))
	require.Equal(t, "General", general.Name(""))
}
