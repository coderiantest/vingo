package vingo

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// -------------------- Helpers / utilities --------------------

// lookup: dot notation support for map/struct
func lookup(data map[string]interface{}, path string) (interface{}, bool) {
	// if path is literal string "..." or number or boolean, don't treat as lookup
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, false
	}
	// quoted string?
	if (strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"")) || (strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'")) {
		unq, err := strconv.Unquote(p)
		if err == nil {
			return unq, true
		}
	}
	// numeric literal?
	if i, err := strconv.Atoi(p); err == nil {
		return i, true
	}
	if f, err := strconv.ParseFloat(p, 64); err == nil {
		return f, true
	}
	if p == "true" {
		return true, true
	}
	if p == "false" {
		return false, true
	}

	var cur interface{} = data
	parts := strings.Split(p, ".")
	for _, seg := range parts {
		switch node := cur.(type) {
		case map[string]interface{}:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		default:
			rv := reflect.ValueOf(cur)
			switch rv.Kind() {
			case reflect.Map:
				if rv.Type().Key().Kind() == reflect.String {
					mv := rv.MapIndex(reflect.ValueOf(seg))
					if !mv.IsValid() {
						return nil, false
					}
					cur = mv.Interface()
				} else {
					return nil, false
				}
			case reflect.Struct:
				f := rv.FieldByName(seg)
				if f.IsValid() {
					cur = f.Interface()
				} else {
					// try method? (not implemented)
					return nil, false
				}
			default:
				return nil, false
			}
		}
	}
	return cur, true
}

func lookupVal(data map[string]interface{}, path string) interface{} {
	v, _ := lookup(data, path)
	return v
}

func shallowCopyMap(m map[string]interface{}) map[string]interface{} {
	n := make(map[string]interface{}, len(m)+4)
	for k, v := range m {
		n[k] = v
	}
	return n
}

// -------------------- Expression Evaluator (basit) --------------------
//
// Supports:
// - Comparisons: ==, !=, >, <, >=, <=
// - Logical: and, or (left-to-right, no operator precedence beyond that)
// - Parentheses not supported in this simple evaluator (could be added)
// - Left and right operands can be identifiers (dot notation), quoted strings, numbers, booleans.

var compOpRe = regexp.MustCompile(`\s*(==|!=|>=|<=|>|<)\s*`)

func evalCondition(expr string, data map[string]interface{}) (bool, error) {
	// split by " and " / " or " preserving order
	// implement left-to-right evaluation
	tokens := splitLogical(expr)
	if len(tokens) == 0 {
		// treat empty as false
		return false, nil
	}
	// tokens like: [cond, op, cond, op, cond...], where op is "and"/"or"
	// evaluate first cond
	res, err := evalSimpleCond(strings.TrimSpace(tokens[0]), data)
	if err != nil {
		return false, err
	}
	i := 1
	for i < len(tokens)-0 {
		op := strings.TrimSpace(tokens[i])
		nextExpr := strings.TrimSpace(tokens[i+1])
		nextRes, err := evalSimpleCond(nextExpr, data)
		if err != nil {
			return false, err
		}
		if op == "and" {
			res = res && nextRes
		} else if op == "or" {
			res = res || nextRes
		} else {
			return false, fmt.Errorf("unknown logical operator %s", op)
		}
		i += 2
		if i >= len(tokens) {
			break
		}
	}
	return res, nil
}

func splitLogical(expr string) []string {
	// naive split: find " and " and " or " tokens
	parts := []string{}
	cur := ""
	low := strings.TrimSpace(expr)
	words := strings.Fields(low)
	// rebuild by scanning tokens
	i := 0
	for i < len(words) {
		w := words[i]
		if w == "and" || w == "or" {
			parts = append(parts, strings.TrimSpace(cur))
			parts = append(parts, w)
			cur = ""
		} else {
			if cur == "" {
				cur = w
			} else {
				cur += " " + w
			}
		}
		i++
	}
	if cur != "" {
		parts = append(parts, strings.TrimSpace(cur))
	}
	return parts
}

func evalSimpleCond(cond string, data map[string]interface{}) (bool, error) {
	// If condition contains comparison operator -> split
	if compOpRe.MatchString(cond) {
		// loc := compOpRe.FindStringIndex(cond)
		op := compOpRe.FindStringSubmatch(cond)[1]
		parts := compOpRe.Split(cond, 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid comparison in '%s'", cond)
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		lv, lok := lookup(data, left)
		if !lok {
			// try literal
			lv = literalFromString(left)
		}
		rv, rok := lookup(data, right)
		if !rok {
			rv = literalFromString(right)
		}
		return compareValues(lv, rv, op)
	}
	// no operator => truthy check of the expression (variable or literal)
	v, ok := lookup(data, cond)
	if ok {
		return condTruthy(v), nil
	}
	// maybe it's literal
	v2 := literalFromString(cond)
	return condTruthy(v2), nil
}

func evalConditionWithValue(condExpr string, value interface{}, data map[string]interface{}) (bool, error) {
	// For switch-case convenience: if condExpr is a literal or simple comparison referencing 'value' or '.' shorthand
	// We'll replace occurrences of "value" or "." with actual value by injecting into data map as special var "__switch__"
	tmp := shallowCopyMap(data)
	tmp["__switch__"] = value
	// allow shorthand: if condExpr equals plain string/number, compare with value
	// But to reuse evalSimpleCond, we accept expressions like "__switch__ == 5" or simply "5" (then compare)
	// If condExpr has no operator, treat as equality to value.
	if compOpRe.MatchString(condExpr) {
		// eval normally but with lookup resolving identifiers possibly
		return evalCondition(condExpr, tmp)
	}
	// no operator: compare value stringified to condExpr literal or to evaluated lookup
	// try literal
	lit := literalFromString(strings.TrimSpace(condExpr))
	// compare value vs lit
	ok, err := compareValues(value, lit, "==")
	if err == nil && ok {
		return true, nil
	}
	// try comparing string form
	if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", lit) {
		return true, nil
	}
	// else try evaluating cond as expression with __switch__ variable
	res, err := evalCondition(condExpr, tmp)
	if err == nil {
		return res, nil
	}
	return false, nil
}

func literalFromString(s string) interface{} {
	s = strings.TrimSpace(s)
	// quoted string
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) || (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		unq, err := strconv.Unquote(s)
		if err == nil {
			return unq
		}
		return s[1 : len(s)-1]
	}
	// bool
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	// int
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	// float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// fallback string
	return s
}

func compareValues(a interface{}, b interface{}, op string) (bool, error) {
	// first try numeric comparison
	af, aIsNum := toFloat(a)
	bf, bIsNum := toFloat(b)
	if aIsNum && bIsNum {
		switch op {
		case "==":
			return af == bf, nil
		case "!=":
			return af != bf, nil
		case ">":
			return af > bf, nil
		case "<":
			return af < bf, nil
		case ">=":
			return af >= bf, nil
		case "<=":
			return af <= bf, nil
		}
	}
	// boolean
	if ab, ok := a.(bool); ok {
		if bb, ok2 := b.(bool); ok2 {
			switch op {
			case "==":
				return ab == bb, nil
			case "!=":
				return ab != bb, nil
			}
		}
	}
	// string compare
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	switch op {
	case "==":
		return as == bs, nil
	case "!=":
		return as != bs, nil
	case ">":
		return as > bs, nil
	case "<":
		return as < bs, nil
	case ">=":
		return as >= bs, nil
	case "<=":
		return as <= bs, nil
	}
	return false, fmt.Errorf("unsupported comparison between %T and %T", a, b)
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int8:
		return float64(t), true
	case int16:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint8:
		return float64(t), true
	case uint16:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	default:
		// try parse from string
		if s := fmt.Sprintf("%v", v); s != "" {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// -------------------- Truthy --------------------

func condTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t != ""
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0
	case float32, float64:
		return reflect.ValueOf(v).Float() != 0
	default:
		// slices/maps: non-empty => true
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len() > 0
		default:
			return true
		}
	}
}
