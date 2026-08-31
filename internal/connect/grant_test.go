package connect

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// useMemoryStore forces the no-Redis path and hands back a clean store, so the
// tests never depend on a Redis being up or on each other's leftovers.
func useMemoryStore(t *testing.T) *memoryStore {
	t.Helper()
	prev := common.RedisEnabled
	common.RedisEnabled = false
	fresh := &memoryStore{items: map[string]memoryItem{}}
	memStoreOnce.Do(func() {})
	prevInst := memStoreInst
	memStoreInst = fresh
	t.Cleanup(func() {
		common.RedisEnabled = prev
		memStoreInst = prevInst
	})
	return fresh
}

func TestConnectGrant_IssueThenRedeem(t *testing.T) {
	useMemoryStore(t)

	token, err := Issue(Grant{UserID: 7, TokenID: 42, Tools: []string{ToolCodex}})
	require.NoError(t, err)
	require.Len(t, token, tokenLength)

	got, err := Redeem(token)
	require.NoError(t, err)
	require.Equal(t, 7, got.UserID)
	require.Equal(t, 42, got.TokenID)
	require.Equal(t, []string{ToolCodex}, got.Tools)
	require.NotZero(t, got.IssuedAt)
}

// Single use is the whole reason the command is safe to copy around: whatever
// ends up in a screenshot is already spent.
func TestConnectGrant_SecondRedeemFails(t *testing.T) {
	useMemoryStore(t)

	token, err := Issue(Grant{UserID: 1, TokenID: 1, Tools: []string{ToolClaudeCode}})
	require.NoError(t, err)

	_, err = Redeem(token)
	require.NoError(t, err)

	_, err = Redeem(token)
	require.ErrorIs(t, err, ErrGrantNotFound)
}

func TestConnectGrant_ExpiredIsRejected(t *testing.T) {
	store := useMemoryStore(t)

	token, err := Issue(Grant{UserID: 1, TokenID: 1, Tools: []string{ToolClaudeCode}})
	require.NoError(t, err)

	// Age it past its TTL rather than sleeping fifteen minutes.
	item := store.items[redisKeyPrefix+token]
	item.expiresAt = time.Now().Add(-time.Second)
	store.items[redisKeyPrefix+token] = item

	_, err = Redeem(token)
	require.ErrorIs(t, err, ErrGrantNotFound)
}

// A token is spent even when it turns out to be expired, so an attacker cannot
// keep a known-expired string around to probe with.
func TestConnectGrant_ExpiredTokenIsAlsoConsumed(t *testing.T) {
	store := useMemoryStore(t)

	token, err := Issue(Grant{UserID: 1, TokenID: 1, Tools: []string{ToolClaudeCode}})
	require.NoError(t, err)
	item := store.items[redisKeyPrefix+token]
	item.expiresAt = time.Now().Add(-time.Second)
	store.items[redisKeyPrefix+token] = item

	_, _ = Redeem(token)
	require.NotContains(t, store.items, redisKeyPrefix+token)
}

// Malformed input must not reach the store — a path parameter should not be
// usable to probe key space.
func TestConnectGrant_MalformedTokenRejected(t *testing.T) {
	useMemoryStore(t)

	for _, token := range []string{
		"", "short", "way-too-long-to-be-a-token",
		"AAAAAAAAA0", // 0 is not in the alphabet (confusable with O)
		"AAAAAAAAA1", // nor is 1 (confusable with I)
		"aaaaaaaaaa", // lowercase is not in the alphabet
		"connect_grant:AAAAAAAAAA",
	} {
		_, err := Redeem(token)
		require.ErrorIs(t, err, ErrGrantNotFound, "token %q should be rejected", token)
	}
}

func TestConnectGrant_TokensAreDistinct(t *testing.T) {
	useMemoryStore(t)

	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		token, err := NewToken()
		require.NoError(t, err)
		require.False(t, seen[token], "NewToken repeated %q", token)
		seen[token] = true
	}
}

// Concurrent redemptions of one token: exactly one may win. Two winners would
// mean the key was served twice from a token meant to be spent once.
func TestConnectGrant_ConcurrentRedeemHasOneWinner(t *testing.T) {
	useMemoryStore(t)

	token, err := Issue(Grant{UserID: 3, TokenID: 9, Tools: []string{ToolOpenCode}})
	require.NoError(t, err)

	const racers = 16
	results := make(chan error, racers)
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		go func() {
			<-start
			_, err := Redeem(token)
			results <- err
		}()
	}
	close(start)

	wins := 0
	for i := 0; i < racers; i++ {
		if <-results == nil {
			wins++
		}
	}
	require.Equal(t, 1, wins, "exactly one redemption may succeed")
}

func TestConnectGrant_NormalizeTools(t *testing.T) {
	t.Run("keeps catalogue order, not request order", func(t *testing.T) {
		got := NormalizeTools([]string{ToolGeminiCLI, ToolClaudeCode})
		require.Equal(t, []string{ToolClaudeCode, ToolGeminiCLI}, got)
	})
	t.Run("drops unknown tools rather than failing", func(t *testing.T) {
		got := NormalizeTools([]string{"cursor", ToolCodex, "zed"})
		require.Equal(t, []string{ToolCodex}, got)
	})
	t.Run("de-duplicates", func(t *testing.T) {
		got := NormalizeTools([]string{ToolCodex, ToolCodex, ToolCodex})
		require.Equal(t, []string{ToolCodex}, got)
	})
	t.Run("empty stays empty", func(t *testing.T) {
		require.Empty(t, NormalizeTools(nil))
		require.Empty(t, NormalizeTools([]string{"nope"}))
	})
}
