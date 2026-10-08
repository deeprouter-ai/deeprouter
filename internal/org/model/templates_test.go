package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// PRD §5 (decision D14, finalised in D23): 创意生成包 = 图片 + 视频 + 对话；coding 包 =
// 编程. Written out by hand, so a change to the templates is a change someone
// meant to make.
func TestPolicyTemplates_AreTheTwoThePRDDefines(t *testing.T) {
	require.Equal(t, []PolicyTemplate{
		{Key: "creative", Purposes: []string{"image", "video", "chat"}},
		{Key: "coding", Purposes: []string{"coding"}},
	}, PolicyTemplates)

	creative, ok := FindPolicyTemplate("creative")
	require.True(t, ok)
	require.Equal(t, []string{"image", "video", "chat"}, creative.Purposes)

	// "No template" is not a template, and neither is anything else.
	for _, key := range []string{"", "Creative", "chat", "all"} {
		_, ok := FindPolicyTemplate(key)
		require.False(t, ok, key)
	}
}
