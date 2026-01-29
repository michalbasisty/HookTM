package cli

import "strings"

// NormalizeArgs makes the CLI more forgiving by allowing flags to appear after
// positional arguments (e.g. `hooktm listen 8080 --forward localhost:3000`).
//
// urfave/cli uses Go's standard flag parsing which stops at the first non-flag
// token, so we normalize to `hooktm listen --forward ... 8080` for known commands.
func NormalizeArgs(argv []string) []string {
	if len(argv) < 2 {
		return argv
	}

	cmd := argv[1]
	switch cmd {
	case "listen":
		return normalizeCommand(argv, cmdFlags{
			valueFlags: map[string]bool{
				"--forward": true,
			},
		})
	case "show":
		return normalizeCommand(argv, cmdFlags{
			valueFlags: map[string]bool{
				"--format": true,
			},
		})
	case "replay":
		return normalizeCommand(argv, cmdFlags{
			valueFlags: map[string]bool{
				"--to":    true,
				"--patch": true,
				"--last":  true,
			},
			boolFlags: map[string]bool{
				"--dry-run": true,
				"--json":    true,
			},
		})
	case "list":
		return normalizeCommand(argv, cmdFlags{
			valueFlags: map[string]bool{
				"--limit":    true,
				"--provider": true,
				"--status":   true,
				"--search":   true,
			},
			boolFlags: map[string]bool{
				"--json": true,
			},
		})
	case "codegen":
		return normalizeCommand(argv, cmdFlags{
			valueFlags: map[string]bool{
				"--lang": true,
			},
		})
	default:
		return argv
	}
}

type cmdFlags struct {
	valueFlags map[string]bool
	boolFlags  map[string]bool
}

func normalizeCommand(argv []string, flags cmdFlags) []string {
	// Keep: argv[0]=exe, argv[1]=cmd
	if len(argv) <= 2 {
		return argv
	}
	// Preserve global flags in position 1.. before cmd? We don't handle that here;
	// MVP uses env vars or flags before command for globals.

	var (
		out     []string
		cmdOut  []string
		flagOut []string
		argsOut []string
	)
	out = append(out, argv[0])
	cmdOut = append(cmdOut, argv[1])

	rest := argv[2:]
	for i := 0; i < len(rest); i++ {
		tok := rest[i]
		// --flag=value form
		if strings.HasPrefix(tok, "--") && strings.Contains(tok, "=") {
			name := tok[:strings.Index(tok, "=")]
			if flags.valueFlags[name] || flags.boolFlags[name] {
				flagOut = append(flagOut, tok)
				continue
			}
		}

		if flags.boolFlags[tok] {
			flagOut = append(flagOut, tok)
			continue
		}
		if flags.valueFlags[tok] {
			// take next token as value if present
			flagOut = append(flagOut, tok)
			if i+1 < len(rest) {
				flagOut = append(flagOut, rest[i+1])
				i++
			}
			continue
		}

		// Not a recognized flag: treat as positional argument.
		argsOut = append(argsOut, tok)
	}

	out = append(out, cmdOut...)
	out = append(out, flagOut...)
	out = append(out, argsOut...)
	return out
}
