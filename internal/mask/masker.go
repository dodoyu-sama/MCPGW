package mask

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/dodoyu-sama/mcpgw/internal/config"
)

const DefaultMaskChar = "****"

type Rule struct {
	Name     string
	Pattern  []*regexp.Regexp
	Fields   []string
	MaskChar string
}

type Masker struct {
	rules []Rule
}

// New compiles the configured rules into a Masker.
func New(rules []config.MaskRule) (*Masker, error) {
	m := &Masker{}
	for _, r := range rules {
		rule := Rule{
			Name:     r.Name,
			Fields:   r.Fields,
			MaskChar: r.MaskChar,
		}
		if rule.MaskChar == "" {
			rule.MaskChar = DefaultMaskChar
		}
		for _, p := range r.Patterns {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("rule %q: compile pattern %q: %w", r.Name, p, err)
			}
			rule.Pattern = append(rule.Pattern, re)
		}
		m.rules = append(m.rules, rule)
	}
	return m, nil
}

// Mask returns a deep-copied, masked version of params.
func (m *Masker) Mask(params map[string]interface{}) (map[string]interface{}, error) {
	out := deepCopy(params)
	if out == nil {
		return map[string]interface{}{}, nil
	}
	m.maskValue(out, "")
	return out, nil
}

func (m *Masker) maskValue(v interface{}, path string) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, v2 := range val {
			if m.matchField(k) {
				val[k] = m.maskFieldValue(v2)
			} else {
				val[k] = m.maskValue(v2, path+"."+k)
			}
		}
		return val
	case []interface{}:
		for i, item := range val {
			val[i] = m.maskValue(item, fmt.Sprintf("%s[%d]", path, i))
		}
		return val
	case string:
		return m.maskString(val)
	default:
		return v
	}
}

func (m *Masker) matchField(name string) bool {
	for _, rule := range m.rules {
		for _, f := range rule.Fields {
			if f == name {
				return true
			}
		}
	}
	return false
}

func (m *Masker) maskFieldValue(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		if masked := m.maskString(s); masked != s {
			return masked
		}
	}
	return DefaultMaskChar
}

func (m *Masker) maskString(s string) string {
	for _, rule := range m.rules {
		for _, re := range rule.Pattern {
			if re.MatchString(s) {
				return re.ReplaceAllString(s, rule.MaskChar)
			}
		}
	}
	return s
}

func deepCopy(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return src
	}
	var dst map[string]interface{}
	if err := json.Unmarshal(data, &dst); err != nil {
		return src
	}
	return dst
}
