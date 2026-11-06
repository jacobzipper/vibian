// Command vibian is the Vibian CLI: onboarding, the agent launcher, the
// ledger, snapshots and undo, and the doctor.
package main

import (
	"fmt"
	"os"
	"strings"
)

var version = "dev"

const usage = `vibian: an OS run by your coding agent

  vibian onboard                      first-boot setup (re-runnable)
  vibian agent [run|set <id>|list]    launch or pick your agent
  vibian powers [root|confirm]        change what the agent may do
  vibian snap "why"                   snapshot root and home before a change
  vibian log add "summary" [flags]    record a change (closes the last snap)
  vibian log [tail|show <id>|search <words>]
  vibian history [-n N] [--json]      ledger entries and snapshot pairs, newest last
  vibian undo [<ledger-id>|--last|--pair <n>|--to <snapshot>]
  vibian export [DIR] [--publish]     this setup as a shareable rice repo, secrets scrubbed
  vibian apply <github:user/repo|dir> your agent applies someone's rice (--scripts-only: no agent)
  vibian doctor [--json]              health check
  vibian mcp                          the vibian-system MCP server (stdio)
  vibian hook turn-start|turn-end     per-turn snapshots (harness hooks run this)
  vibian version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "onboard":
		err = cmdOnboard(args)
	case "agent":
		err = cmdAgent(args)
	case "powers":
		err = cmdPowers(args)
	case "snap":
		err = cmdSnap(args)
	case "log":
		err = cmdLog(args)
	case "history":
		err = cmdHistory(args)
	case "undo":
		err = cmdUndo(args)
	case "export":
		err = cmdExport(args)
	case "apply":
		err = cmdApply(args)
	case "doctor":
		err = cmdDoctor(args)
	case "mcp":
		err = cmdMCP(args)
	case "hook":
		err = cmdHook(args)
	case "version", "--version":
		fmt.Println("vibian", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "vibian: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vibian:", strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
