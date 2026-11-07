//@ pragma IconTheme Papirus-Dark
// Vibian desk, from the vibian playbook "quickshell-desk": a taskbar on every
// screen, a start menu, and desktop widgets on the first screen. Hyprland
// starts it (qs -c vibian-desk). Settings: desk.json, here.
//   qs -c vibian-desk ipc call desk start        open or close the start menu
//   qs -c vibian-desk ipc call desk ask "text"   ask the agent in a terminal
// Quickshell docs: https://quickshell.org/docs/
import QtQuick
import Quickshell
import Quickshell.Io

ShellRoot {
    Variants {
        model: Quickshell.screens
        Taskbar {}
    }
    StartMenu {}
    ClockWidget {}
    AskWidget {}
    KeysWidget {}
    SystemWidget {}
    ChangesWidget {}

    IpcHandler {
        target: "desk"
        function start(): void { Desk.startOpen = !Desk.startOpen }
        function ask(prompt: string): void { Desk.ask(prompt) }
    }
}
