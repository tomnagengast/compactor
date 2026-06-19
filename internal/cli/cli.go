package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/tomnagengast/compactor/internal/capsule"
	"github.com/tomnagengast/compactor/internal/hookio"
	"github.com/tomnagengast/compactor/internal/store"
)

const usage = `compactor

Progressive disclosure for agent compaction.

Usage:
  compactor --help
  compactor --version
  compactor hook <agent> <phase>

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

	return fmt.Errorf("unknown command: %s\n\n%s", args[0], usage)
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
