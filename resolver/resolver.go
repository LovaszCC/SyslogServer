package resolver

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	positiveTTL = 1 * time.Hour
	negativeTTL = 5 * time.Minute
	lookupTimeout = 500 * time.Millisecond
)

type entry struct {
	name      string
	expiresAt time.Time
}

type Resolver struct {
	cache sync.Map
	r     *net.Resolver
}

func New() *Resolver {
	return &Resolver{r: net.DefaultResolver}
}

func (r *Resolver) Lookup(ctx context.Context, ip string) string {
	if ip == "" {
		return ""
	}

	if v, ok := r.cache.Load(ip); ok {
		e := v.(entry)
		if time.Now().Before(e.expiresAt) {
			return e.name
		}
	}

	lctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()

	names, err := r.r.LookupAddr(lctx, ip)
	var name string
	ttl := negativeTTL
	if err == nil && len(names) > 0 {
		name = strings.TrimSuffix(names[0], ".")
		ttl = positiveTTL
	}

	r.cache.Store(ip, entry{name: name, expiresAt: time.Now().Add(ttl)})
	return name
}
