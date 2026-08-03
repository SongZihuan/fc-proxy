package rules

import (
	"net/http"
	"testing"
)

// mustCompile 预编译规则正则，测试中手动构造 []Rule 时必须调用。
func mustCompile(t *testing.T, rules []Rule) []Rule {
	t.Helper()
	for i := range rules {
		if err := rules[i].Compile(); err != nil {
			t.Fatalf("compile rule %d: %v", i, err)
		}
	}
	return rules
}

func TestCompileInvalidRegex(t *testing.T) {
	r := &Rule{PathRegex: "([bad"}
	if err := r.Compile(); err == nil {
		t.Error("非法正则应报错")
	}
}

func TestMatchFirstRuleWins(t *testing.T) {
	rules := mustCompile(t, []Rule{
		{PathRegex: "^/api", SetHeaders: map[string]string{"X-Rule": "api"}},
		{SetHeaders: map[string]string{"X-Rule": "fallback"}},
	})

	if r := MatchFirst(rules, "/api/private"); r.SetHeaders["X-Rule"] != "api" {
		t.Errorf("/api/private 应命中第一条规则，得到 %+v", r)
	}
	if r := MatchFirst(rules, "/other"); r.SetHeaders["X-Rule"] != "fallback" {
		t.Errorf("/other 应命中兜底规则，得到 %+v", r)
	}
}

func TestMatchNoRule(t *testing.T) {
	rules := mustCompile(t, []Rule{{PathRegex: "^/api"}})
	if r := MatchFirst(rules, "/other"); r != nil {
		t.Errorf("无兜底规则时 /other 不应命中，得到 %+v", r)
	}
}

func TestApplyDeleteThenSet(t *testing.T) {
	rule := &Rule{
		SetHeaders:    map[string]string{"X-A": "new"},
		DeleteHeaders: []string{"X-A", "X-B"},
	}
	h := http.Header{"X-A": {"old"}, "X-B": {"gone"}, "X-C": {"keep"}}
	Apply(rule, h)

	if got := h.Get("X-A"); got != "new" {
		t.Errorf("X-A 应被覆盖为 new，得到 %q", got)
	}
	if _, ok := h["X-B"]; ok {
		t.Errorf("X-B 应被删除")
	}
	if got := h.Get("X-C"); got != "keep" {
		t.Errorf("X-C 不应受影响，得到 %q", got)
	}
}

func TestApplyNilRule(t *testing.T) {
	h := http.Header{"X-A": {"old"}}
	Apply(nil, h)
	if got := h.Get("X-A"); got != "old" {
		t.Errorf("nil 规则不应修改头部，得到 %q", got)
	}
}
