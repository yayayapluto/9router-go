package executor

import (
	"bytes"
	"slices"
	"testing"

	json "encoding/json/v2"
)

// TestStripStrictUpstreamFields pins the OVH sanitize pass: vLLM/OMP thinking
// controls are removed, a developer role becomes system, and reasoning_content
// survives (DeepSeek 400s without it on tool-call turns).
func TestStripStrictUpstreamFields(t *testing.T) {
	t.Run("drops controls and rewrites role", func(t *testing.T) {
		body := []byte(`{"model":"m","preserve_thinking":true,"chat_template_kwargs":{"a":1},"enable_thinking":false,"messages":[{"role":"developer","content":"x"},{"role":"assistant","thought":"t","reasoning_content":"keep me","content":"y"}]}`)
		out := stripStrictUpstreamFields(body)

		var m map[string]any
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatalf("output is not JSON: %v (%s)", err, out)
		}
		for _, name := range strictUpstreamDroppedFields {
			if _, ok := m[name]; ok {
				t.Errorf("%s still present in %s", name, out)
			}
		}
		msgs, _ := m["messages"].([]any)
		if len(msgs) != 2 {
			t.Fatalf("messages changed shape: %s", out)
		}
		dev, _ := msgs[0].(map[string]any)
		if dev["role"] != "system" {
			t.Errorf("developer role not rewritten: %s", out)
		}
		assistant, _ := msgs[1].(map[string]any)
		if _, ok := assistant["thought"]; ok {
			t.Errorf("thought alias survived: %s", out)
		}
		if assistant["reasoning_content"] != "keep me" {
			t.Errorf("reasoning_content lost: %s", out)
		}
	})

	t.Run("untouched body returned byte-identical", func(t *testing.T) {
		body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
		if out := stripStrictUpstreamFields(body); !bytes.Equal(out, body) {
			t.Errorf("body rewritten: %s", out)
		}
	})

	t.Run("invalid JSON returned unchanged", func(t *testing.T) {
		body := []byte(`{"preserve_thinking":`)
		if out := stripStrictUpstreamFields(body); !bytes.Equal(out, body) {
			t.Errorf("invalid body rewritten: %s", out)
		}
	})
}

// FuzzStripStrictUpstreamFields keeps the sanitiser total: unparseable bodies
// come back byte-identical, and a sanitised body always parses and carries
// none of the dropped fields.
func FuzzStripStrictUpstreamFields(f *testing.F) {
	f.Add([]byte(`{"messages":[{"role":"developer","content":"x"}],"enable_thinking":true}`))
	f.Add([]byte(`{"messages":[]}`))
	f.Add([]byte(`not json`))
	f.Add([]byte{0x7b, 0x00})

	f.Fuzz(func(t *testing.T, body []byte) {
		out := stripStrictUpstreamFields(body)
		if slices.Equal(out, body) {
			return
		}
		var parsed map[string]any
		if err := json.Unmarshal(out, &parsed); err != nil {
			t.Fatalf("sanitised body is not JSON: %v", err)
		}
		for _, name := range strictUpstreamDroppedFields {
			if _, ok := parsed[name]; ok {
				t.Fatalf("field %q survived sanitising", name)
			}
		}
	})
}
