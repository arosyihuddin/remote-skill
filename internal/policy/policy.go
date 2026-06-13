// Package policy enforces opt-in allow/deny rules for handlers.
//
// All policies are configured on the agent (laptop) side. Empty rule sets
// mean "allow everything" (preserving current behavior).
package policy

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrDenied is returned when a request is rejected by policy.
var ErrDenied = errors.New("denied by policy")

// Policy holds compiled rules.
type Policy struct {
	allowCmd []*regexp.Regexp
	denyCmd  []*regexp.Regexp
	denyPath []*regexp.Regexp
	allowGUI bool
}

// New compiles a policy from raw config strings. Errors on bad regex.
func New(allowCmd, denyCmd, denyPath []string, allowGUI bool) (*Policy, error) {
	p := &Policy{allowGUI: allowGUI}
	for _, pat := range allowCmd {
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("ALLOW_CMD %q: %w", pat, err)
		}
		p.allowCmd = append(p.allowCmd, re)
	}
	for _, pat := range denyCmd {
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("DENY_CMD %q: %w", pat, err)
		}
		p.denyCmd = append(p.denyCmd, re)
	}
	for _, pat := range denyPath {
		re, err := tryGlobToRegex(pat)
		if err != nil {
			return nil, fmt.Errorf("DENY_PATH %q: %w", pat, err)
		}
		p.denyPath = append(p.denyPath, re)
	}
	return p, nil
}

// CheckCmd inspects argv (or shell line). The first element is treated as
// the program name. For shell mode the shell line is checked as a string.
func (p *Policy) CheckCmd(cmd []string, shell bool) error {
	if p == nil {
		return nil
	}
	target := ""
	if shell && len(cmd) > 0 {
		target = strings.Join(cmd, " ")
	} else if len(cmd) > 0 {
		target = cmd[0]
	}
	for _, re := range p.denyCmd {
		if re.MatchString(target) {
			return fmt.Errorf("%w: command matches DENY_CMD %q", ErrDenied, re.String())
		}
	}
	if len(p.allowCmd) > 0 {
		ok := false
		for _, re := range p.allowCmd {
			if re.MatchString(target) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("%w: command not in ALLOW_CMD", ErrDenied)
		}
	}
	return nil
}

// CheckPath rejects paths matching any DenyPath glob. Paths are first cleaned
// and made absolute; ** matches recursively.
func (p *Policy) CheckPath(path string) error {
	if p == nil || len(p.denyPath) == 0 {
		return nil
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		abs = path
	}
	for _, re := range p.denyPath {
		if re.MatchString(abs) {
			return fmt.Errorf("%w: path matches DENY_PATH", ErrDenied)
		}
	}
	return nil
}

// CheckGUI gates all GUI actions.
func (p *Policy) CheckGUI() error {
	if p == nil || p.allowGUI {
		return nil
	}
	return fmt.Errorf("%w: GUI actions disabled (ALLOW_GUI=false)", ErrDenied)
}

func tryGlobToRegex(pattern string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				sb.WriteString(".*")
				i++
			} else {
				sb.WriteString("[^/]*")
			}
		case '?':
			sb.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}
