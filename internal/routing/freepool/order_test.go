package freepool

import (
	"reflect"
	"testing"
)

func e(provider, model string) Entry { return Entry{Provider: provider, Model: model} }

func TestOrder(t *testing.T) {
	// Every table row uses one provider unless the row says otherwise, so the
	// ordering shown comes from the capability rules rather than the
	// cross-upstream rule.
	tests := []struct {
		name       string
		entries    []Entry
		caps       map[string]Caps
		needTools  bool
		needVision bool
		parked     map[string]bool
		want       []Entry
	}{
		{
			name: "context below ninety percent is rejected",
			entries: []Entry{
				e("x", "origin"),
				e("x", "small"),
				e("x", "exact90"),
			},
			caps: map[string]Caps{
				"origin":  {ContextWindow: 10000},
				"small":   {ContextWindow: 8900},
				"exact90": {ContextWindow: 9000},
			},
			want: []Entry{e("x", "origin"), e("x", "exact90"), e("x", "small")},
		},
		{
			name: "context at ninety percent is kept",
			entries: []Entry{
				e("x", "origin"),
				e("x", "exact90"),
			},
			caps: map[string]Caps{
				"origin":  {ContextWindow: 10000},
				"exact90": {ContextWindow: 9000},
			},
			want: []Entry{e("x", "origin"), e("x", "exact90")},
		},
		{
			name: "larger context preferred over smaller",
			entries: []Entry{
				e("x", "origin"),
				e("x", "small"),
				e("x", "large"),
			},
			caps: map[string]Caps{
				"origin": {ContextWindow: 1000},
				"small":  {ContextWindow: 900},
				"large":  {ContextWindow: 2000},
			},
			want: []Entry{e("x", "origin"), e("x", "large"), e("x", "small")},
		},
		{
			name: "reasoning mismatch loses to a matching entry",
			entries: []Entry{
				e("x", "origin"),
				e("x", "no-reasoning"),
				e("x", "reasoning"),
			},
			caps: map[string]Caps{
				"origin":       {Reasoning: true, ContextWindow: 10000},
				"no-reasoning": {ContextWindow: 20000},
				"reasoning":    {Reasoning: true, ContextWindow: 9000},
			},
			want: []Entry{e("x", "origin"), e("x", "reasoning"), e("x", "no-reasoning")},
		},
		{
			name: "needVision drops a model without vision",
			entries: []Entry{
				e("x", "origin"),
				e("x", "blind"),
				e("x", "sighted"),
			},
			caps: map[string]Caps{
				"origin":  {Vision: true},
				"blind":   {},
				"sighted": {Vision: true},
			},
			needVision: true,
			want:       []Entry{e("x", "origin"), e("x", "sighted")},
		},
		{
			name: "needTools drops a model without tools",
			entries: []Entry{
				e("x", "origin"),
				e("x", "no-tools"),
				e("x", "tools"),
			},
			caps: map[string]Caps{
				"origin":   {Tools: true},
				"no-tools": {},
				"tools":    {Tools: true},
			},
			needTools: true,
			want:      []Entry{e("x", "origin"), e("x", "tools")},
		},
		{
			name: "all filtered returns the original list",
			entries: []Entry{
				e("x", "a"),
				e("x", "b"),
				e("x", "c"),
			},
			caps: map[string]Caps{
				"a": {},
				"b": {},
				"c": {},
			},
			needTools: true,
			want:      []Entry{e("x", "a"), e("x", "b"), e("x", "c")},
		},
		{
			name: "all parked returns the unparked-filter result",
			entries: []Entry{
				e("x", "a"),
				e("x", "b"),
			},
			caps:   map[string]Caps{},
			parked: map[string]bool{"a": true, "b": true},
			want:   []Entry{e("x", "a"), e("x", "b")},
		},
		{
			name: "second pick avoids the provider of the first",
			entries: []Entry{
				e("p1", "a"),
				e("p1", "b"),
				e("p2", "c"),
			},
			caps: map[string]Caps{},
			want: []Entry{e("p1", "a"), e("p2", "c"), e("p1", "b")},
		},
		{
			name: "parked model is skipped",
			entries: []Entry{
				e("x", "parked"),
				e("x", "b"),
				e("x", "c"),
			},
			caps:   map[string]Caps{},
			parked: map[string]bool{"parked": true},
			want:   []Entry{e("x", "b"), e("x", "c")},
		},
		{
			name: "unknown context is never rejected",
			entries: []Entry{
				e("x", "origin"),
				e("x", "unknown"),
			},
			caps: map[string]Caps{
				"origin":  {ContextWindow: 10000},
				"unknown": {},
			},
			want: []Entry{e("x", "origin"), e("x", "unknown")},
		},
		{
			name:    "no entries unchanged",
			entries: nil,
			caps:    map[string]Caps{},
			want:    nil,
		},
		{
			name:    "one entry unchanged",
			entries: []Entry{e("x", "only")},
			caps:    map[string]Caps{},
			want:    []Entry{e("x", "only")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := func(provider, model string) Caps {
				return tt.caps[model]
			}
			parked := func(provider, model string) bool {
				return tt.parked[model]
			}
			got := Order(tt.entries, tt.needTools, tt.needVision, meta, parked)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Order() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderNilMetaAndParked(t *testing.T) {
	entries := []Entry{e("p1", "a"), e("p1", "b"), e("p2", "c")}
	got := Order(entries, false, false, nil, nil)
	want := []Entry{e("p1", "a"), e("p2", "c"), e("p1", "b")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Order() = %v, want %v", got, want)
	}
}
