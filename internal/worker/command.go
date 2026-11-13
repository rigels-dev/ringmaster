package worker

import (
	"errors"
	"fmt"
	"strings"
)

// SplitCommand turns a command line into the argument list Ringmaster runs.
//
// Commands are executed directly, never through a shell, but arguments are
// split the way a shell would: whitespace separates them, and single or
// double quotes keep an argument together, so
//
//	python -c "import wordstat; print(wordstat.__version__)"
//
// is three arguments. Nothing is expanded: there are no variables, globs or
// substitutions. Anything that only a shell could do (pipes, &&, ;,
// redirects, $(...), backticks) outside quotes is an error, as is command
// substitution inside double quotes, where a shell would have run it.
func SplitCommand(line string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		inArg   bool
		quote   rune // 0, '\'' or '"'
	)
	runes := []rune(line)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch quote {
		case '\'':
			if r == '\'' {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue

		case '"':
			switch {
			case r == '"':
				quote = 0
			case r == '\\' && i+1 < len(runes) && strings.ContainsRune(`"\$`+"`", runes[i+1]):
				i++
				current.WriteRune(runes[i])
			case r == '`', r == '$' && i+1 < len(runes) && runes[i+1] == '(':
				return nil, errors.New("command substitution ($(...) or backticks) needs a shell, and commands run without one")
			default:
				current.WriteRune(r)
			}
			continue
		}

		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inArg {
				args = append(args, current.String())
				current.Reset()
				inArg = false
			}
		case r == '\'' || r == '"':
			quote = r
			inArg = true
		case r == '\\':
			if i+1 >= len(runes) {
				return nil, errors.New("the command ends with a backslash")
			}
			i++
			current.WriteRune(runes[i])
			inArg = true
		case r == '|' || r == '&' || r == ';' || r == '<' || r == '>' || r == '`':
			return nil, fmt.Errorf("'%c' needs a shell, and commands run without one; use one plain command, or put the character inside quotes if it belongs to an argument", r)
		case r == '$' && i+1 < len(runes) && runes[i+1] == '(':
			return nil, errors.New("command substitution ($(...)) needs a shell, and commands run without one")
		default:
			current.WriteRune(r)
			inArg = true
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("a %c quote is never closed", quote)
	}
	if inArg {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, errors.New("the command is empty")
	}
	return args, nil
}

// Interpreters that take a program as the argument after one of these flags.
var inlineCodeFlags = map[string]string{
	"python": "-c", "python3": "-c", "sh": "-c", "bash": "-c",
	"node": "-e", "ruby": "-e", "perl": "-e",
}

// UnquotedInlineCode spots a program passed to an interpreter without
// quotes, such as `python -c import wordstat`: the interpreter is given only
// the first word ("import") as its program and fails with a syntax error
// that no amount of work on the task can fix. It returns "" when args is not
// such a command.
func UnquotedInlineCode(args []string) string {
	if len(args) < 4 {
		return ""
	}
	flag, known := inlineCodeFlags[args[0]]
	if !known {
		return ""
	}
	for i := 1; i < len(args)-2; i++ {
		if args[i] != flag {
			if !strings.HasPrefix(args[i], "-") {
				// A script or module name: what follows are its arguments, not the interpreter's
				return ""
			}
			continue
		}
		code := args[i+1]
		if isBareWord(code) {
			return fmt.Sprintf("the program after %s %s must be ONE quoted argument, but only %q would be run as the program; write it as %s %s \"%s\"",
				args[0], flag, code, args[0], flag, strings.Join(args[i+1:], " "))
		}
		return ""
	}
	return ""
}

func isBareWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_') {
			return false
		}
	}
	return true
}
