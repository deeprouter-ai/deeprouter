package model

import "github.com/QuantumNous/new-api/common"

// InvalidateTokenCacheByKey drops the cached copy of the key stored under a key
// value, so the next request made with that value is answered from the
// database. It is the door to cacheDeleteToken for code outside this package:
// internal/org changes organization keys in its own transactions and has to
// tell the cache once they are committed — a rotated or frozen key must stop
// working at once, not when its cache entry expires (meta-repo
// docs/enterprise-org-prd.md §3).
//
// It reports whether there is a cache at all. Without a connected Redis there
// is none: nothing was dropped, and the caller has nothing to come back for.
func InvalidateTokenCacheByKey(key string) (cached bool, err error) {
	if !common.RedisEnabled || common.RDB == nil || key == "" {
		return false, nil
	}
	return true, cacheDeleteToken(key)
}
