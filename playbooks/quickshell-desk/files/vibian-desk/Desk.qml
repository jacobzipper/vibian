// Shared settings, colours and helpers. desk.json, next to this file, holds
// the settings (user.sh writes it; changes apply as soon as the file is saved):
//   accent, background  6 hex digits
//   terminal            foot, kitty or alacritty
//   mod                 the Hyprland modifier, for the keys widget
//   widgets             which widgets to show: clock, ask, keys, system, changes
// theme.json, also here, is the look, for themes and rices. The playbook
// creates it empty and never writes it again. Every key is optional:
//   accent, background, fg, dim, faint, red, yellow, green   6 hex digits
//   font, mono, display   font families: text, keys, the clock and titles
//   clock                 the big clock's font, if not display's
//   radius                corner radius of the cards in px (10)
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Hyprland

Singleton {
    id: desk

    readonly property string home: Quickshell.env("HOME")
    property var conf: ({})
    property var theme: ({})
    function hex(key, fallback) {
        const v = theme[key] || conf[key] || fallback
        return /^[0-9a-fA-F]{6}$/.test(v) ? "#" + v : "#" + fallback
    }

    readonly property color accent: hex("accent", "7aa2f7")
    readonly property color bg: hex("background", "16161e")
    readonly property color card: Qt.rgba(bg.r, bg.g, bg.b, 0.96)
    readonly property color raised: Qt.lighter(bg, 1.9)
    readonly property color line: Qt.lighter(bg, 2.6)
    readonly property color fg: hex("fg", "c0caf5")
    readonly property color dim: hex("dim", "a9b1d6")
    readonly property color faint: hex("faint", "737aa2")
    readonly property color red: hex("red", "f7768e")
    readonly property color yellow: hex("yellow", "e0af68")
    readonly property color green: hex("green", "9ece6a")
    readonly property string font: theme.font || "Inter"
    readonly property string mono: theme.mono || "monospace"
    readonly property string display: theme.display || font
    readonly property string clockFont: theme.clock || display
    readonly property real radius: theme.radius >= 0 ? theme.radius : 10
    // a corner radius from the default look, scaled to the theme's
    function r(px) { return Math.round(px * radius / 10) }
    readonly property string terminal: conf.terminal || "foot"
    readonly property string mod: conf.mod || "SUPER"

    // the screen the desktop widgets live on, and the one with the focus
    readonly property var primary: Quickshell.screens.length ? Quickshell.screens[0] : null
    readonly property var focused: {
        const m = Hyprland.focusedMonitor
        return (m && Quickshell.screens.find(s => s.name === m.name)) || primary
    }

    property bool startOpen: false

    function on(widget) { return !conf.widgets || conf.widgets.indexOf(widget) >= 0 }

    FileView {
        path: Quickshell.shellDir + "/desk.json"
        watchChanges: true
        onFileChanged: reload()
        onLoaded: { try { desk.conf = JSON.parse(text()) } catch (e) { console.warn("desk.json:", e) } }
    }
    FileView {
        path: Quickshell.shellDir + "/theme.json"
        watchChanges: true
        onFileChanged: reload()
        onLoaded: { try { desk.theme = JSON.parse(text()) } catch (e) { console.warn("theme.json:", e) } }
    }

    function run(argv) { Quickshell.execDetached(argv) }
    function inTerminal(argv) { run((terminal === "alacritty" ? ["alacritty", "-e"] : [terminal]).concat(argv)) }
    // empty prompt: just open the agent
    function ask(prompt) {
        inTerminal(prompt ? ["vibian", "agent", "run", "--prompt", prompt] : ["vibian", "agent"])
    }
    function icon(name) { return Quickshell.iconPath(name || "application-x-executable", "application-x-executable") }
    // reading applications.values makes bindings re-run once the entries load
    function entryFor(appId) {
        return appId && DesktopEntries.applications.values.length ? DesktopEntries.heuristicLookup(appId) : null
    }
}
