package vingo

import (
	"fmt"
	"regexp"
	"strings"
)

type TokenType int

const (
	TText TokenType = iota
	TVar
	TIf
	TElseIf
	TElse
	TEndIf
	TFor
	TEndFor
	TSwitch
	TCase
	TDefault
	TEndSwitch
)

type Token struct {
	Type    TokenType
	Value   string // for Var: expression or name; for If/For/Switch/Case: expression / raw
	Default string // for Var default literal (if provided)
	Raw     string // raw tag text
}

var (
	varPattern       = regexp.MustCompile(`^\s*(\w+(?:\.\w+)*)(?:\s*\|\s*"(.*?)")?\s*$`)
	ifPattern        = regexp.MustCompile(`^if\s+(.+)$`)
	elseifPattern    = regexp.MustCompile(`^elseif\s+(.+)$`)
	elsePattern      = regexp.MustCompile(`^else$`)
	endifPattern     = regexp.MustCompile(`^/if$`)
	forPattern       = regexp.MustCompile(`^for\s+(.+)\s+in\s+(.+)$`)
	endforPattern    = regexp.MustCompile(`^/for$`)
	switchPattern    = regexp.MustCompile(`^switch\s+(.+)$`)
	casePattern      = regexp.MustCompile(`^case\s+(.+)$`)
	defaultPattern   = regexp.MustCompile(`^default$`)
	endswitchPattern = regexp.MustCompile(`^/switch$`)
)

func tokenize(input string) []*Token {
	var tokens []*Token
	parts := strings.Split(input, "<{")

	for _, part := range parts {
		if part == "" {
			continue
		}

		sub := strings.SplitN(part, "}>", 2)
		if len(sub) == 2 {
			tag := strings.TrimSpace(sub[0])
			rest := sub[1]

			switch {
			case ifPattern.MatchString(tag):
				m := ifPattern.FindStringSubmatch(tag)
				tokens = append(tokens, &Token{Type: TIf, Value: m[1], Raw: tag})
			case elseifPattern.MatchString(tag):
				m := elseifPattern.FindStringSubmatch(tag)
				tokens = append(tokens, &Token{Type: TElseIf, Value: m[1], Raw: tag})
			case elsePattern.MatchString(tag):
				tokens = append(tokens, &Token{Type: TElse, Raw: tag})
			case endifPattern.MatchString(tag):
				tokens = append(tokens, &Token{Type: TEndIf, Raw: tag})
			case forPattern.MatchString(tag):
				m := forPattern.FindStringSubmatch(tag)
				// m[1] could be "idx, item" or "item"
				tokens = append(tokens, &Token{Type: TFor, Value: strings.TrimSpace(m[1]) + ":" + strings.TrimSpace(m[2]), Raw: tag})
			case endforPattern.MatchString(tag):
				tokens = append(tokens, &Token{Type: TEndFor, Raw: tag})
			case switchPattern.MatchString(tag):
				m := switchPattern.FindStringSubmatch(tag)
				tokens = append(tokens, &Token{Type: TSwitch, Value: m[1], Raw: tag})
			case casePattern.MatchString(tag):
				m := casePattern.FindStringSubmatch(tag)
				tokens = append(tokens, &Token{Type: TCase, Value: m[1], Raw: tag})
			case defaultPattern.MatchString(tag):
				tokens = append(tokens, &Token{Type: TDefault, Raw: tag})
			case endswitchPattern.MatchString(tag):
				tokens = append(tokens, &Token{Type: TEndSwitch, Raw: tag})
			case varPattern.MatchString(tag):
				m := varPattern.FindStringSubmatch(tag)
				tokens = append(tokens, &Token{Type: TVar, Value: m[1], Default: m[2], Raw: tag})
			default:
				// treat as text containing the tag (unknown tag kept)
				tokens = append(tokens, &Token{Type: TText, Value: "<{" + tag + "}>", Raw: tag})
			}

			if rest != "" {
				tokens = append(tokens, &Token{Type: TText, Value: rest})
			}
		} else {
			// trailing text without closing tag
			tokens = append(tokens, &Token{Type: TText, Value: "<{" + part})
		}
	}

	return tokens
}

// -------------------- compile (tokens -> AST nodes) --------------------

func compileTokens(tokens []*Token) ([]Node, error) {
	nodes := []Node{}
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		switch t.Type {
		case TText:
			nodes = append(nodes, &TextNode{Text: t.Value})
			i++
		case TVar:
			// parse filters from t.Raw maybe in future; currently only default supported.
			filters := []string{}
			// if user wants filters like <{ var | upper }>, varPattern must be extended.
			nodes = append(nodes, &VarNode{Name: t.Value, Default: t.Default, Filters: filters})
			i++
		case TIf:
			ifNode, ni, err := parseIf(tokens, i)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, ifNode)
			i = ni
		case TFor:
			forNode, ni, err := parseFor(tokens, i)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, forNode)
			i = ni
		case TSwitch:
			switchNode, ni, err := parseSwitch(tokens, i)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, switchNode)
			i = ni
		default:
			return nil, fmt.Errorf("unexpected token %v at position %d (raw: %s)", t.Type, i, t.Raw)
		}
	}
	return nodes, nil
}

func parseIf(tokens []*Token, start int) (*IfNode, int, error) {
	// tokens[start] is TIf
	root := &IfNode{}
	branches := []IfBranch{{Expr: tokens[start].Value, Body: []Node{}}}
	elseBody := []Node{}
	currentBody := &branches[0].Body
	depth := 0

	i := start + 1
	for i < len(tokens) {
		t := tokens[i]
		switch t.Type {
		case TIf:
			// nested if: append token as part of body by compiling nested structure
			nested, ni, err := parseIf(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			*currentBody = append(*currentBody, nested)
			i = ni
			continue
		case TEndIf:
			if depth == 0 {
				// finish
				root.Branches = branches
				root.Else = elseBody
				return root, i + 1, nil
			}
			depth--
			*currentBody = append(*currentBody, &TextNode{Text: t.Value})
		case TElseIf:
			if depth == 0 {
				branches = append(branches, IfBranch{Expr: t.Value, Body: []Node{}})
				currentBody = &branches[len(branches)-1].Body
				i++
				continue
			}
			*currentBody = append(*currentBody, &TextNode{Text: t.Value})
		case TElse:
			if depth == 0 {
				elseBody = []Node{}
				currentBody = &elseBody
				i++
				continue
			}
			*currentBody = append(*currentBody, &TextNode{Text: t.Value})
		case TFor:
			fnode, ni, err := parseFor(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			*currentBody = append(*currentBody, fnode)
			i = ni
			continue
		case TSwitch:
			snode, ni, err := parseSwitch(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			*currentBody = append(*currentBody, snode)
			i = ni
			continue
		default:
			// Text or Var
			switch t.Type {
			case TText:
				*currentBody = append(*currentBody, &TextNode{Text: t.Value})
			case TVar:
				*currentBody = append(*currentBody, &VarNode{Name: t.Value, Default: t.Default})
			default:
				return nil, 0, fmt.Errorf("unexpected token inside if: %v", t.Type)
			}
			i++
		}
	}
	return nil, 0, fmt.Errorf("unclosed if starting at token %d", start)
}

func parseFor(tokens []*Token, start int) (*ForNode, int, error) {
	// tokens[start] is TFor with Value like "idx, item:listExpr" or "item:listExpr"
	parts := strings.SplitN(tokens[start].Value, ":", 2)
	if len(parts) != 2 {
		return nil, 0, fmt.Errorf("invalid for tag: %s", tokens[start].Raw)
	}
	left := strings.TrimSpace(parts[0])
	listExpr := strings.TrimSpace(parts[1])

	indexVar := ""
	itemVar := ""
	if strings.Contains(left, ",") {
		p := strings.SplitN(left, ",", 2)
		indexVar = strings.TrimSpace(p[0])
		itemVar = strings.TrimSpace(p[1])
	} else {
		itemVar = left
	}

	node := &ForNode{IndexVar: indexVar, ItemVar: itemVar, ListExpr: listExpr, Body: []Node{}}
	i := start + 1
	depth := 0
	for i < len(tokens) {
		t := tokens[i]
		switch t.Type {
		case TFor:
			// nested for
			nf, ni, err := parseFor(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			node.Body = append(node.Body, nf)
			i = ni
			continue
		case TEndFor:
			if depth == 0 {
				return node, i + 1, nil
			}
			depth--
			node.Body = append(node.Body, &TextNode{Text: t.Value})
		case TIf:
			ifn, ni, err := parseIf(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			node.Body = append(node.Body, ifn)
			i = ni
			continue
		case TSwitch:
			sn, ni, err := parseSwitch(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			node.Body = append(node.Body, sn)
			i = ni
			continue
		default:
			switch t.Type {
			case TText:
				node.Body = append(node.Body, &TextNode{Text: t.Value})
			case TVar:
				node.Body = append(node.Body, &VarNode{Name: t.Value, Default: t.Default})
			default:
				return nil, 0, fmt.Errorf("unexpected token in for: %v", t.Type)
			}
			i++
		}
	}
	return nil, 0, fmt.Errorf("unclosed for starting at token %d", start)
}

func parseSwitch(tokens []*Token, start int) (*SwitchNode, int, error) {
	node := &SwitchNode{Expr: tokens[start].Value, Cases: []SwitchCase{}, Default: []Node{}}
	i := start + 1
	depth := 0
	currentCond := ""
	currentBody := []Node{}

	flushCase := func() {
		if currentCond != "" {
			node.Cases = append(node.Cases, SwitchCase{Cond: currentCond, Body: currentBody})
		} else if currentBody != nil && len(currentBody) > 0 {
			node.Default = currentBody
		}
		currentCond = ""
		currentBody = []Node{}
	}

	for i < len(tokens) {
		t := tokens[i]
		switch t.Type {
		case TSwitch:
			// nested
			nn, ni, err := parseSwitch(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			currentBody = append(currentBody, nn)
			i = ni
			continue
		case TEndSwitch:
			if depth == 0 {
				flushCase()
				return node, i + 1, nil
			}
			depth--
			currentBody = append(currentBody, &TextNode{Text: t.Value})
		case TCase:
			if depth == 0 {
				// finish previous
				flushCase()
				currentCond = t.Value
				currentBody = []Node{}
				i++
				continue
			}
			currentBody = append(currentBody, &TextNode{Text: t.Value})
		case TDefault:
			if depth == 0 {
				flushCase()
				currentCond = ""
				currentBody = []Node{}
				i++
				continue
			}
			currentBody = append(currentBody, &TextNode{Text: t.Value})
		case TIf:
			in, ni, err := parseIf(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			currentBody = append(currentBody, in)
			i = ni
			continue
		case TFor:
			fn, ni, err := parseFor(tokens, i)
			if err != nil {
				return nil, 0, err
			}
			currentBody = append(currentBody, fn)
			i = ni
			continue
		default:
			switch t.Type {
			case TText:
				currentBody = append(currentBody, &TextNode{Text: t.Value})
			case TVar:
				currentBody = append(currentBody, &VarNode{Name: t.Value, Default: t.Default})
			default:
				return nil, 0, fmt.Errorf("unexpected token in switch: %v", t.Type)
			}
			i++
		}
	}
	return nil, 0, fmt.Errorf("unclosed switch starting at token %d", start)
}
