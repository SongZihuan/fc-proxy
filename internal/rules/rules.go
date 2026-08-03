// Package rules 实现请求/响应头改写规则：按 URL path 顺序匹配，
// 第一条命中的规则先生效，命中后先删除再覆盖头部。
package rules

import (
	"fmt"
	"net/http"
	"regexp"
)

// Rule 是一条请求或响应规则：按 path_regex 匹配 URL path，命中后先删除再覆盖头部。
// PathRegex 为空表示兜底规则，匹配一切路径。只有第一条命中的规则生效。
type Rule struct {
	PathRegex     string            `yaml:"path_regex"`
	SetHeaders    map[string]string `yaml:"set_headers"`
	DeleteHeaders []string          `yaml:"delete_headers"`

	regex *regexp.Regexp // 预编译的正则，nil 表示兜底规则
}

// Compile 预编译 PathRegex；PathRegex 为空时作为兜底规则（regex 保持 nil）。
func (r *Rule) Compile() error {
	if r.PathRegex == "" {
		r.regex = nil
		return nil
	}
	re, err := regexp.Compile(r.PathRegex)
	if err != nil {
		return fmt.Errorf("非法正则 %q: %w", r.PathRegex, err)
	}
	r.regex = re
	return nil
}

// Match 报告规则是否命中 path；兜底规则（regex 为 nil）匹配一切。
func (r *Rule) Match(path string) bool {
	return r.regex == nil || r.regex.MatchString(path)
}

// MatchFirst 返回第一条命中 path 的规则；规则按顺序匹配。
// 没有任何规则命中时返回 nil（不修改头部）。
func MatchFirst(rules []Rule, path string) *Rule {
	for i := range rules {
		if rules[i].Match(path) {
			return &rules[i]
		}
	}
	return nil
}

// Apply 先删除 DeleteHeaders，再覆盖 SetHeaders（同名头以 set 为准）。
func Apply(r *Rule, h http.Header) {
	if r == nil {
		return
	}
	for _, name := range r.DeleteHeaders {
		h.Del(name)
	}
	for name, value := range r.SetHeaders {
		h.Set(name, value)
	}
}
