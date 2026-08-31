// Package connect issues the one-time tokens behind the key page's one-click
// setup block, and redeems them for a ready-to-run install script.
//
// Why a token at all, rather than putting the key in the command: the command
// is meant to be copied, so it lands in clipboards, terminal scrollback,
// screen recordings and screenshots. What travels through all that is a token
// that dies after one use or fifteen minutes; the API key itself is injected
// server-side at redemption and never appears in a URL.
package connect

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	// GrantTTL is how long an issued token stays redeemable (PRD §4.1).
	GrantTTL = 15 * time.Minute

	// tokenLength is a balance between being readable in a command and being
	// unguessable. 10 chars of the alphabet below is ~51 bits, and a guess only
	// has 15 minutes and one shot.
	tokenLength = 10

	// Digits and uppercase letters minus the pairs that are hard to tell apart
	// in a terminal font (0/O, 1/I). A user retyping a command from a screenshot
	// should not be defeated by the alphabet.
	tokenAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

	redisKeyPrefix = "connect_grant:"
)

// ErrGrantNotFound covers every reason a token cannot be redeemed — unknown,
// already used, or expired. They are deliberately indistinguishable to the
// caller: telling an attacker which of the three it was is free information,
// and a user only ever needs the same answer ("get a fresh command").
var ErrGrantNotFound = errors.New("connect: token is invalid, already used, or expired")

// Grant is what one token stands for: permission to fetch exactly one user's
// key, for exactly the tools they ticked, once.
//
// It stores the token's row ID rather than the key itself, so the key is read
// from the database at redemption. A grant sitting in Redis is therefore not
// itself a secret worth stealing, and a key regenerated in the meantime is not
// handed out stale.
type Grant struct {
	UserID   int      `json:"user_id"`
	TokenID  int      `json:"token_id"`
	Tools    []string `json:"tools"`
	IssuedAt int64    `json:"issued_at"`
}

// store is the minimum a grant store has to do. Two implementations exist
// because the gateway runs both with and without Redis.
type store interface {
	// put saves a grant under key with a TTL.
	put(key string, g Grant, ttl time.Duration) error
	// claim atomically returns the grant and removes it, so two concurrent
	// redemptions of the same token cannot both succeed.
	claim(key string) (Grant, error)
}

var (
	memStoreOnce sync.Once
	memStoreInst *memoryStore
)

// currentStore picks Redis when it is configured and memory otherwise. Chosen
// per call rather than once at init: common.RedisEnabled is settled during
// startup, and tests flip it.
func currentStore() store {
	if common.RedisEnabled {
		return redisStore{}
	}
	memStoreOnce.Do(func() { memStoreInst = &memoryStore{items: map[string]memoryItem{}} })
	return memStoreInst
}

// NewToken returns a fresh random token string.
func NewToken() (string, error) {
	var sb strings.Builder
	sb.Grow(tokenLength)
	max := big.NewInt(int64(len(tokenAlphabet)))
	for i := 0; i < tokenLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(tokenAlphabet[n.Int64()])
	}
	return sb.String(), nil
}

// Issue stores a grant and returns the token that redeems it.
func Issue(g Grant) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	g.IssuedAt = time.Now().Unix()
	if err := currentStore().put(redisKeyPrefix+token, g, GrantTTL); err != nil {
		return "", err
	}
	return token, nil
}

// Redeem consumes a token and returns what it granted. A token is spent the
// moment it is redeemed, whether or not the caller goes on to succeed —
// retrying with a new command is cheap, and re-serving a key is not.
func Redeem(token string) (Grant, error) {
	if !validToken(token) {
		return Grant{}, ErrGrantNotFound
	}
	return currentStore().claim(redisKeyPrefix + token)
}

// validToken rejects anything that cannot be one of ours before it reaches the
// store, so a path parameter cannot be used to probe key space.
func validToken(token string) bool {
	if len(token) != tokenLength {
		return false
	}
	for i := 0; i < len(token); i++ {
		if !strings.ContainsRune(tokenAlphabet, rune(token[i])) {
			return false
		}
	}
	return true
}

// --- Redis-backed store -----------------------------------------------------

type redisStore struct{}

func (redisStore) put(key string, g Grant, ttl time.Duration) error {
	payload, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return common.RedisSet(key, string(payload), ttl)
}

func (redisStore) claim(key string) (Grant, error) {
	payload, err := common.RedisGet(key)
	if err != nil || payload == "" {
		return Grant{}, ErrGrantNotFound
	}
	// DEL reports how many keys it removed, so it doubles as the atomic claim:
	// of two concurrent redemptions exactly one sees 1. Using DEL's count
	// rather than GETDEL keeps this working on Redis older than 6.2.
	removed, err := common.RDB.Del(context.Background(), key).Result()
	if err != nil || removed == 0 {
		return Grant{}, ErrGrantNotFound
	}
	var g Grant
	if err := json.Unmarshal([]byte(payload), &g); err != nil {
		return Grant{}, ErrGrantNotFound
	}
	return g, nil
}

// --- In-memory store --------------------------------------------------------

type memoryItem struct {
	grant     Grant
	expiresAt time.Time
}

// memoryStore is the no-Redis path. Grants are short-lived and worthless after
// use, so losing them on restart is fine — the user just copies the command again.
type memoryStore struct {
	mu    sync.Mutex
	items map[string]memoryItem
}

func (m *memoryStore) put(key string, g Grant, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evictExpiredLocked()
	m.items[key] = memoryItem{grant: g, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (m *memoryStore) claim(key string) (Grant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[key]
	if !ok {
		return Grant{}, ErrGrantNotFound
	}
	delete(m.items, key)
	if time.Now().After(item.expiresAt) {
		return Grant{}, ErrGrantNotFound
	}
	return item.grant, nil
}

// evictExpiredLocked keeps the map from growing without bound. Sweeping on
// write is enough: entries are only added here, and the map is small.
func (m *memoryStore) evictExpiredLocked() {
	now := time.Now()
	for k, v := range m.items {
		if now.After(v.expiresAt) {
			delete(m.items, k)
		}
	}
}
