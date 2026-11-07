#!/bin/bash
# Quickshell desk playbook check: the hyprland playbook's checks, then
# Quickshell and the desk. Prints one line per check and VERIFY OK or
# VERIFY FAILED.
set -u
here=$(cd "$(dirname "$0")/.." && pwd)
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
desk=$cfg/quickshell/vibian-desk
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
bash "$here/../hyprland/scripts/verify.sh" | grep -v '^VERIFY '
[ "${PIPESTATUS[0]}" = 0 ] || bad=1
check "Quickshell built and installed" "/usr/local/bin/quickshell --version | grep -q '^Quickshell ' && test -x /usr/local/bin/qs"
check "Quickshell finds all its libraries" "! ldd /usr/local/bin/quickshell | grep -q 'not found'"
check "Qt Quick modules installed" "test -d /usr/lib/x86_64-linux-gnu/qt6/qml/QtQuick/Layouts -o -d /usr/lib/aarch64-linux-gnu/qt6/qml/QtQuick/Layouts"
check "Hyprland starts the desk" "grep -qx 'exec-once = qs -c vibian-desk' $cfg/hypr/hyprland.conf"
check "SUPER+Space opens the start menu" "grep -qx '\\\$menu = qs -c vibian-desk ipc call desk start' $cfg/hypr/hyprland.conf"
check "the desk's files" "test -f $desk/shell.qml && test -f $desk/Taskbar.qml && test -f $desk/Desk.qml"
check "desk.json is valid" "jq -e '.accent and (.widgets | type == \"array\")' $desk/desk.json"
check "tickers.json is valid" "jq -e '.tickers | type == \"array\"' $desk/tickers.json"
check "the stocks script parses" "python3 -I -c 'import ast, sys; ast.parse(open(sys.argv[1]).read())' $desk/stocks.py"
check "theme.json is valid" "jq -e 'type == \"object\"' $desk/theme.json"
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
