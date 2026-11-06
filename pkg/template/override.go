package template

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// OverrideValues validates values against the template context and, when
// every key is valid, replaces the matching defaults and switches the
// template to non-interactive execution. On error the template is untouched.
func (t *dirTemplate) OverrideValues(values map[string]interface{}) error {
	ctxt, _ := deepCopy(t.Context).(map[string]interface{})
	if ctxt == nil {
		ctxt = map[string]interface{}{}
	}

	if errs := override(ctxt, values, ""); len(errs) > 0 {
		return errors.Join(errs...)
	}

	t.Context = ctxt
	t.ShouldUseDefaults = true

	return nil
}

// EffectiveValues returns the values the template renders with default
// binding: every choice list is reduced to its first item, groups keep
// their structure.
func (t *dirTemplate) EffectiveValues() map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range t.Context {
		out[k] = effective(v)
	}

	return out
}

func effective(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		if len(v) == 0 {
			return nil
		}
		return deepCopy(v[0])
	case map[string]interface{}:
		m := make(map[string]interface{}, len(v))
		for k, child := range v {
			m[k] = effective(child)
		}
		return m
	}

	return v
}

func override(dst, src map[string]interface{}, path string) []error {
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var errs []error
	for _, key := range keys {
		keyPath := key
		if path != "" {
			keyPath = path + "." + key
		}

		val := src[key]
		def, ok := dst[key]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown key %q", keyPath))
			continue
		}

		switch def := def.(type) {
		case []interface{}:
			found := false
			for _, opt := range def {
				if reflect.DeepEqual(opt, val) {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, fmt.Errorf("invalid option for %q: got %s, valid options: [%s]",
					keyPath, formatValue(val), formatOptions(def)))
				continue
			}
			dst[key] = val
		case map[string]interface{}:
			child, ok := val.(map[string]interface{})
			if !ok {
				errs = append(errs, fmt.Errorf("type mismatch for %q: expected object, got %s", keyPath, jsonKind(val)))
				continue
			}
			errs = append(errs, override(def, child, keyPath)...)
		default:
			if want, got := jsonKind(def), jsonKind(val); want != got {
				errs = append(errs, fmt.Errorf("type mismatch for %q: expected %s, got %s", keyPath, want, got))
				continue
			}
			dst[key] = val
		}
	}

	return errs
}

func jsonKind(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64:
		return "number"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "list"
	}

	return fmt.Sprintf("%T", v)
}

func formatValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}

	return fmt.Sprintf("%v", v)
}

func formatOptions(opts []interface{}) string {
	parts := make([]string, len(opts))
	for i, o := range opts {
		parts[i] = fmt.Sprintf("%v", o)
	}

	return strings.Join(parts, ", ")
}

func deepCopy(v interface{}) interface{} {
	switch v := v.(type) {
	case map[string]interface{}:
		if v == nil {
			return map[string]interface{}(nil)
		}
		m := make(map[string]interface{}, len(v))
		for k, child := range v {
			m[k] = deepCopy(child)
		}
		return m
	case []interface{}:
		s := make([]interface{}, len(v))
		for i, child := range v {
			s[i] = deepCopy(child)
		}
		return s
	}

	return v
}
