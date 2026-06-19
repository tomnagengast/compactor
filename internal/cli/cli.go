package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tomnagengast/compactor/internal/capsule"
	"github.com/tomnagengast/compactor/internal/hookio"
	"github.com/tomnagengast/compactor/internal/install"
	"github.com/tomnagengast/compactor/internal/reference"
	"github.com/tomnagengast/compactor/internal/snippet"
	"github.com/tomnagengast/compactor/internal/store"
)

const usage = `compactor

Progressive disclosure for agent compaction.

Usage:
  compactor --help
  compactor --version
  compactor resolve <ref-or-path> [--cwd <path>] [--max-bytes <n>]
  compactor hook <agent> <phase>
  compactor hooks snippet <agent> [--binary <path>]
  compactor hooks install <agent> [--scope project|user] [--binary <path>] [--write]
  compactor hooks uninstall <agent> [--scope project|user] [--binary <path>] [--write]

Agents:
  claude
  codex

Hook phases:
  precompact
  postcompact
  inject
`

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, version string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		return nil
	}

	if args[0] == "--version" || args[0] == "-v" || args[0] == "version" {
		fmt.Fprintf(stdout, "compactor %s\n", version)
		return nil
	}

	if args[0] == "hook" {
		return runHook(args[1:], stdin, stdout)
	}
	if args[0] == "hooks" {
		return runHooks(args[1:], stdout)
	}
	if args[0] == "resolve" {
		return runResolve(args[1:], stdout)
	}

	return fmt.Errorf("unknown command: %s\n\n%s", args[0], usage)
}

func runResolve(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: compactor resolve <ref-or-path> [--cwd <path>] [--max-bytes <n>]")
	}

	ref := args[0]
	cwd := ""
	maxBytes := reference.DefaultMaxBytes
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--cwd":
			if i+1 >= len(args) {
				return fmt.Errorf("--cwd requires a path")
			}
			cwd = args[i+1]
			i++
		case "--max-bytes":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-bytes requires a positive integer")
			}
			parsed, err := reference.ParseMaxBytes(args[i+1])
			if err != nil {
				return err
			}
			maxBytes = parsed
			i++
		default:
			return fmt.Errorf("unknown resolve flag: %s", args[i])
		}
	}

	text, err := reference.Resolve(ref, cwd, maxBytes)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, text)
	return err
}

func runHooks(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: compactor hooks <snippet|install> <agent>")
	}

	switch args[0] {
	case "snippet":
		return runHooksSnippet(args[1:], stdout)
	case "install":
		return runHooksInstall(args[1:], stdout)
	case "uninstall":
		return runHooksUninstall(args[1:], stdout)
	default:
		return fmt.Errorf("unknown hooks command: %s", args[0])
	}
}

func runHooksSnippet(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: compactor hooks snippet <agent> [--binary <path>]")
	}

	agent, err := hookio.ParseAgent(args[0])
	if err != nil {
		return err
	}

	binary := "compactor"
	binaryProvided := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--binary":
			if i+1 >= len(args) {
				return fmt.Errorf("--binary requires a path")
			}
			binary = args[i+1]
			binaryProvided = true
			i++
		default:
			return fmt.Errorf("unknown hooks snippet flag: %s", args[i])
		}
	}

	if !binaryProvided {
		if exe, err := os.Executable(); err == nil && exe != "" {
			binary = exe
		}
	}

	text, err := snippet.Hooks(agent, binary)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, text)
	return err
}

func runHooksInstall(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: compactor hooks install <agent> [--scope project|user] [--binary <path>] [--write]")
	}

	agent, err := hookio.ParseAgent(args[0])
	if err != nil {
		return err
	}

	binary := "compactor"
	binaryProvided := false
	scope := install.ScopeProject
	write := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--binary":
			if i+1 >= len(args) {
				return fmt.Errorf("--binary requires a path")
			}
			binary = args[i+1]
			binaryProvided = true
			i++
		case "--scope":
			if i+1 >= len(args) {
				return fmt.Errorf("--scope requires project or user")
			}
			scope = install.Scope(args[i+1])
			i++
		case "--write":
			write = true
		case "--dry-run":
			write = false
		default:
			return fmt.Errorf("unknown hooks install flag: %s", args[i])
		}
	}

	if !binaryProvided {
		if exe, err := os.Executable(); err == nil && exe != "" {
			binary = exe
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	plan, err := install.NewPlan(agent, scope, binary, cwd)
	if err != nil {
		return err
	}
	if write {
		if err := plan.Write(); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "installed hooks for %s at %s\n", agent, plan.Target)
		return nil
	}
	text, err := plan.DryRun()
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, text)
	return err
}

func runHooksUninstall(args []string, stdout io.Writer) error {
	agent, scope, binary, write, err := parseInstallArgs(args, "uninstall")
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	plan, err := install.NewUninstallPlan(agent, scope, binary, cwd)
	if err != nil {
		return err
	}
	if write {
		if err := plan.Write(); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "uninstalled hooks for %s at %s\n", agent, plan.Target)
		return nil
	}
	text, err := plan.DryRun()
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, text)
	return err
}

func parseInstallArgs(args []string, command string) (hookio.Agent, install.Scope, string, bool, error) {
	if len(args) < 1 {
		return "", "", "", false, fmt.Errorf("usage: compactor hooks %s <agent> [--scope project|user] [--binary <path>] [--write]", command)
	}

	agent, err := hookio.ParseAgent(args[0])
	if err != nil {
		return "", "", "", false, err
	}

	binary := "compactor"
	binaryProvided := false
	scope := install.ScopeProject
	write := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--binary":
			if i+1 >= len(args) {
				return "", "", "", false, fmt.Errorf("--binary requires a path")
			}
			binary = args[i+1]
			binaryProvided = true
			i++
		case "--scope":
			if i+1 >= len(args) {
				return "", "", "", false, fmt.Errorf("--scope requires project or user")
			}
			scope = install.Scope(args[i+1])
			i++
		case "--write":
			write = true
		case "--dry-run":
			write = false
		default:
			return "", "", "", false, fmt.Errorf("unknown hooks %s flag: %s", command, args[i])
		}
	}

	if !binaryProvided {
		if exe, err := os.Executable(); err == nil && exe != "" {
			binary = exe
		}
	}
	return agent, scope, binary, write, nil
}

func runHook(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: compactor hook <agent> <phase>")
	}

	agent, err := hookio.ParseAgent(args[0])
	if err != nil {
		return err
	}

	phase, err := hookio.ParsePhase(args[1])
	if err != nil {
		return err
	}

	event, err := hookio.DecodeEvent(stdin, agent)
	if err != nil {
		return hookio.EncodeWarning(stdout, fmt.Sprintf("compactor could not decode hook input: %v", err))
	}

	manager := store.NewManager()
	switch phase {
	case hookio.PhasePreCompact:
		_, err := manager.PreCompact(event)
		if err != nil {
			return hookio.EncodeWarning(stdout, fmt.Sprintf("compactor precompact failed: %v", err))
		}
		return hookio.EncodeContinue(stdout, agent, "", "")
	case hookio.PhasePostCompact:
		result, err := manager.PostCompact(event)
		if err != nil {
			return hookio.EncodeWarning(stdout, fmt.Sprintf("compactor postcompact failed: %v", err))
		}
		_ = result
		return hookio.EncodeContinue(stdout, agent, "", "")
	case hookio.PhaseInject:
		text, err := manager.PendingContext(event)
		if err != nil {
			return hookio.EncodeWarning(stdout, fmt.Sprintf("compactor inject failed: %v", err))
		}
		text = capsule.Trim(text, capsule.DefaultMaxBytes)
		return hookio.EncodeContinue(stdout, agent, event.HookEventName, strings.TrimSpace(text))
	default:
		return fmt.Errorf("unsupported hook phase: %s", phase)
	}
}
