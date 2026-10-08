package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md D16): the keys a
// member was handed, as far as one purpose of the console goes.

// keyLimitedTo writes a key held by holderID whose whitelist is the given
// entries; none at all leaves it free to call every model.
func keyLimitedTo(t *testing.T, db *gorm.DB, org testOrg, holderID int, name string, entries ...string) platformmodel.Token {
	t.Helper()
	key := seedKey(t, db, org, holderID, name)
	if len(entries) > 0 {
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Updates(map[string]any{
			"model_limits_enabled": true,
			"model_limits":         joinEntries(entries),
		}).Error)
	}
	return reloadKey(t, db, key.Id)
}

// joinEntries renders whitelist entries the way tokens.model_limits stores them.
func joinEntries(entries []string) string {
	joined := ""
	for i, entry := range entries {
		if i > 0 {
			joined += ","
		}
		joined += entry
	}
	return joined
}

func TestAllowancesMeet(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		meet bool
	}{
		{"gpt-4o", "gpt-4o", true},
		{"gpt-4o", "gpt-4o-mini", false},
		{"gpt-4o-mini", "gpt-4o", false},
		{"gpt-4o*", "gpt-4o-mini", true},
		{"gpt-4o-mini", "gpt-4o*", true},
		{"claude-*", "gpt-4o", false},
		{"gpt-4o", "claude-*", false},
		// Two rules meet when one covers the other…
		{"claude-*", "claude-sonnet-*", true},
		{"claude-sonnet-*", "claude-*", true},
		{"claude-*", "claude-*", true},
		// …and not when they merely start alike.
		{"claude-sonnet-*", "claude-opus-*", false},
		{"*", "anything", true},
		{"anything", "*", true},
		{"MiniMax-H3", "minimax-h3", false},
	} {
		require.Equal(t, tc.meet, allowancesMeet(tc.a, tc.b), "%s / %s", tc.a, tc.b)
	}
}

// PRD D16: 简易控制台、视频页等自动建 key 的入口对组织成员改用已分配的 key；没有可用
// 的 key 时提示联系管理员 — which is what an empty answer here leads to.
func TestListOwnKeysFor_AreTheMembersKeysThatCanServeThePurpose(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		alice := actorFor(t, db, f.alice.Id)
		// The catalogue of these tests: video and image name models, chat and
		// coding are rules (fixtures_test.go).
		models := fixedCatalogue{purposes: map[string][]string{
			"chat":   {"claude-*", "gpt-4o*", "deeprouter-auto"},
			"coding": {"claude-sonnet-*", "o1*", "deeprouter-auto"},
			"image":  {"dall-e-3"},
			"video":  {"MiniMax-H3", "doubao-seedance-2-0-260128"},
		}}
		names := func(views []OwnKeyView) []string {
			out := []string{}
			for _, view := range views {
				out = append(out, view.Name)
			}
			return out
		}
		ask := func(purpose string) []OwnKeyView {
			views, err := ListOwnKeysFor(db, alice, purpose, models)
			require.NoError(t, err)
			return views
		}

		require.Empty(t, ask("video"), "a member who was handed nothing has nothing to set a tool up with")

		everything := keyLimitedTo(t, db, f.org, f.alice.Id, "everything")
		video := keyLimitedTo(t, db, f.org, f.alice.Id, "one video model", "MiniMax-H3")
		creative := keyLimitedTo(t, db, f.org, f.alice.Id, "creative", "MiniMax-H3", "claude-*", "dall-e-3", "doubao-seedance-2-0-260128", "gpt-4o*", "deeprouter-auto")
		keyLimitedTo(t, db, f.org, f.alice.Id, "coding", "claude-sonnet-*", "o1*", "deeprouter-auto")
		keyLimitedTo(t, db, f.org, f.alice.Id, "hand-picked chat", "gpt-4o-mini")
		// Keys that cannot work right now are never offered…
		withKeyStatus(t, db, keyLimitedTo(t, db, f.org, f.alice.Id, "frozen").Id, common.TokenStatusDisabled)
		expired := keyLimitedTo(t, db, f.org, f.alice.Id, "expired")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", expired.Id).Update("expired_time", time.Now().Add(-time.Hour).Unix()).Error)
		exhausted := keyLimitedTo(t, db, f.org, f.alice.Id, "exhausted")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", exhausted.Id).Update("remain_quota", 0).Error)
		// …nor is anybody else's, nor a personal key.
		keyLimitedTo(t, db, f.org, f.bob.Id, "bob's")
		require.NoError(t, db.Create(&platformmodel.Token{UserId: f.alice.Id, Name: "personal", Key: "personal-key-of-alice", Status: common.TokenStatusEnabled, UnlimitedQuota: true, ExpiredTime: -1}).Error)

		// Newest first. A key that may call every model serves every purpose
		// and is told all of the purpose's models; a limited key the ones it
		// may call.
		require.Equal(t, []OwnKeyView{
			{Id: creative.Id, Name: "creative", Models: []string{"MiniMax-H3", "doubao-seedance-2-0-260128"}},
			{Id: video.Id, Name: "one video model", Models: []string{"MiniMax-H3"}},
			{Id: everything.Id, Name: "everything", Models: []string{"MiniMax-H3", "doubao-seedance-2-0-260128"}},
		}, ask("video"))
		require.Equal(t, []string{"creative", "everything"}, names(ask("image")))
		// Rules meet rules, and a hand-picked model meets the rule covering it.
		require.Equal(t, []string{"hand-picked chat", "coding", "creative", "everything"}, names(ask("chat")))
		require.Equal(t, []string{"coding", "creative", "everything"}, names(ask("coding")))
		// A purpose made of rules names only what it names outright.
		for _, view := range ask("coding") {
			require.Equal(t, []string{"deeprouter-auto"}, view.Models, view.Name)
		}
		require.Equal(t, []string{}, ask("chat")[0].Models, "a hand-picked chat model is not one the purpose names")
		// A purpose nothing is known about is served by no key at all — not
		// even by the one that may call everything.
		require.Empty(t, ask("voice"))
		require.Empty(t, ask("no-such-purpose"))
		require.Empty(t, ask(""))

		// The owner's parked keys are the owner's own.
		parked := keyLimitedTo(t, db, f.org, f.org.owner.Id, "parked")
		views, err := ListOwnKeysFor(db, f.owner, "image", models)
		require.NoError(t, err)
		require.Equal(t, []OwnKeyView{{Id: parked.Id, Name: "parked", Models: []string{"dall-e-3"}}}, views)
	})
}

// The purpose is resolved for the member who asks: what they can be served is
// what their keys are judged against.
func TestListOwnKeysFor_AsksTheCatalogueAboutTheCaller(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		keyLimitedTo(t, db, f.org, f.bob.Id, "bob's")
		asked := map[int]string{}
		views, err := ListOwnKeysFor(db, actorFor(t, db, f.bob.Id), "video", askingCatalogue{asked: asked})
		require.NoError(t, err)
		require.Len(t, views, 1)
		require.Equal(t, map[int]string{f.bob.Id: "video"}, asked)
		require.Equal(t, orgmodel.RoleStaff, roleOf(t, db, f.bob.Id), "it takes no permission at all")
	})
}

// askingCatalogue is a catalogue that notes who a purpose was resolved for.
type askingCatalogue struct {
	asked map[int]string
}

// PurposeModels notes the holder and the purpose, and names one model.
func (c askingCatalogue) PurposeModels(holderID int, purpose string) ([]string, error) {
	c.asked[holderID] = purpose
	return []string{"MiniMax-H3"}, nil
}

// ServableModels is not asked by anything under test here.
func (c askingCatalogue) ServableModels(int) ([]string, error) {
	return nil, nil
}
