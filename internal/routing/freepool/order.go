// Package freepool orders the members of a free-tier combo per request and
// parks upstreams that answered with a retryable failure.
package freepool

// Entry is one free-tier combo member: the provider alias and the wire model id.
type Entry struct{ Provider, Model string }

// Caps describes what a model can do. A zero ContextWindow or MaxOutput means
// "unknown", not "zero": such a model is never rejected or treated as smaller.
type Caps struct {
	Reasoning     bool
	Tools         bool
	Vision        bool
	ContextWindow int
	MaxOutput     int
}

// Meta resolves the capabilities of a model.
type Meta func(provider, model string) Caps

// Parked reports whether a model is currently parked.
type Parked func(provider, model string) bool

// Order returns the entries to try, in order. It never drops a request's only
// remaining option: when a filter would empty the list, the unfiltered list is
// used instead.
func Order(entries []Entry, needTools, needVision bool, meta Meta, parked Parked) []Entry {
	if len(entries) <= 1 {
		return entries
	}

	caps := make([]Caps, len(entries))
	if meta != nil {
		for i, e := range entries {
			caps[i] = meta(e.Provider, e.Model)
		}
	}

	candidates := make([]int, 0, len(entries))
	for i := range entries {
		if needTools && !caps[i].Tools {
			continue
		}
		if needVision && !caps[i].Vision {
			continue
		}
		candidates = append(candidates, i)
	}
	if len(candidates) == 0 {
		candidates = make([]int, len(entries))
		for i := range entries {
			candidates[i] = i
		}
	}

	if parked != nil {
		alive := make([]int, 0, len(candidates))
		for _, i := range candidates {
			if !parked(entries[i].Provider, entries[i].Model) {
				alive = append(alive, i)
			}
		}
		if len(alive) > 0 {
			candidates = alive
		}
	}

	origin := caps[candidates[0]]
	remaining := candidates
	out := make([]Entry, 0, len(candidates))
	lastProvider := ""
	for len(remaining) > 0 {
		pool := remaining
		if lastProvider != "" {
			spread := make([]int, 0, len(pool))
			for _, i := range pool {
				if entries[i].Provider != lastProvider {
					spread = append(spread, i)
				}
			}
			if len(spread) > 0 {
				pool = spread
			}
		}
		if origin.ContextWindow > 0 {
			floor := origin.ContextWindow * 9 / 10
			wide := make([]int, 0, len(pool))
			for _, i := range pool {
				c := caps[i].ContextWindow
				if c > 0 && c < floor {
					continue
				}
				wide = append(wide, i)
			}
			if len(wide) > 0 {
				pool = wide
			}
		}

		best := pool[0]
		for _, i := range pool[1:] {
			if better(caps[i], caps[best], origin) {
				best = i
			}
		}

		out = append(out, entries[best])
		lastProvider = entries[best].Provider
		remaining = remove(remaining, best)
	}
	return out
}

// rank is the comparison tuple for one candidate against the origin. Higher
// reasoning and ctxClass win; lower diff wins; higher maxOut wins.
type rank struct {
	reasoning int
	ctxClass  int
	diff      int
	maxOut    int
}

func rankOf(c, origin Caps) rank {
	r := rank{maxOut: c.MaxOutput}
	if c.Reasoning == origin.Reasoning {
		r.reasoning = 1
	}
	switch {
	case origin.ContextWindow <= 0, c.ContextWindow <= 0:
		// Unknown on either side: not comparable, so never smaller than the
		// origin.
		r.ctxClass = 1
	case c.ContextWindow >= origin.ContextWindow:
		r.ctxClass = 2
		r.diff = c.ContextWindow - origin.ContextWindow
	default:
		r.diff = origin.ContextWindow - c.ContextWindow
	}
	return r
}

func better(c, best, origin Caps) bool {
	a, b := rankOf(c, origin), rankOf(best, origin)
	if a.reasoning != b.reasoning {
		return a.reasoning > b.reasoning
	}
	if a.ctxClass != b.ctxClass {
		return a.ctxClass > b.ctxClass
	}
	if a.diff != b.diff {
		return a.diff < b.diff
	}
	return a.maxOut > b.maxOut
}

func remove(xs []int, v int) []int {
	for i, x := range xs {
		if x == v {
			return append(xs[:i], xs[i+1:]...)
		}
	}
	return xs
}
