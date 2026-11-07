// The start menu (SUPER+Space or the logo): type to find an app, or to ask
// the agent; lock, log out, restart and shut down below.
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Hyprland
import Quickshell.Wayland
import Quickshell.Widgets

PanelWindow {
    id: start
    screen: Desk.focused
    anchors { bottom: true; left: true }
    margins { bottom: 8; left: 8 }
    implicitWidth: 520
    implicitHeight: 600
    color: "transparent"
    visible: Desk.startOpen
    WlrLayershell.namespace: "vibian-start"
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.keyboardFocus: visible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    property string query: ""
    property string confirm: ""   // the power button waiting for a second click

    readonly property var apps: DesktopEntries.applications.values.filter(e => !e.noDisplay)
        .slice().sort((a, b) => a.name.localeCompare(b.name))
    // apps that match, best first, then "ask the agent" (so Return asks when nothing matches)
    readonly property var results: {
        const q = query.trim().toLowerCase()
        if (!q) return apps
        const score = e => {
            const n = e.name.toLowerCase()
            if (n.startsWith(q)) return 0
            if (n.indexOf(q) >= 0) return 1
            // then the start of a word in the generic name or keywords ("browser", "editor")
            const more = ((e.genericName || "") + " " + (e.keywords || []).join(" ")).toLowerCase().split(/[^a-z0-9]+/)
            return more.some(w => w.startsWith(q)) ? 2 : 9
        }
        return apps.map(e => ({ e: e, s: score(e) })).filter(x => x.s < 9)
                   .sort((a, b) => a.s - b.s).map(x => x.e).concat([{ ask: true }])
    }

    function choose(item) {
        const q = query.trim()  // closing the menu clears the query
        Desk.startOpen = false
        if (item.ask) Desk.ask(q)
        else item.execute()
    }
    function power(kind, argv) {
        if (confirm !== kind) { confirm = kind; confirmReset.restart(); return }
        Desk.startOpen = false
        Desk.run(argv)
    }

    onVisibleChanged: {
        search.text = ""; confirm = ""
        if (visible) search.forceActiveFocus()
    }
    Timer { id: confirmReset; interval: 3000; onTriggered: start.confirm = "" }
    HyprlandFocusGrab { windows: [start]; active: start.visible; onCleared: Desk.startOpen = false }

    Rectangle {
        anchors.fill: parent
        color: Desk.bg
        border.color: Desk.line
        radius: Desk.r(12)

        ColumnLayout {
            anchors { fill: parent; margins: 14 }
            spacing: 10

            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 44
                radius: Desk.r(8)
                color: Desk.raised
                border.color: search.activeFocus ? Desk.accent : "transparent"
                TextInput {
                    id: search
                    anchors { fill: parent; leftMargin: 14; rightMargin: 14 }
                    verticalAlignment: TextInput.AlignVCenter
                    color: Desk.fg
                    selectionColor: Desk.accent
                    font.family: Desk.font
                    font.pixelSize: 15
                    clip: true
                    onTextChanged: { start.query = text; list.currentIndex = 0 }
                    Keys.onEscapePressed: Desk.startOpen = false
                    Keys.onDownPressed: list.incrementCurrentIndex()
                    Keys.onUpPressed: list.decrementCurrentIndex()
                    Keys.onReturnPressed: if (start.results.length) start.choose(start.results[Math.max(0, list.currentIndex)])
                    Keys.onEnterPressed: if (start.results.length) start.choose(start.results[Math.max(0, list.currentIndex)])
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !search.text
                        text: "Find an app, or ask the agent anything"
                        color: Desk.faint
                        font: search.font
                    }
                }
            }

            ListView {
                id: list
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                model: start.results
                spacing: 2
                highlightMoveDuration: 0
                boundsBehavior: Flickable.StopAtBounds
                delegate: Rectangle {
                    id: row
                    required property var modelData
                    required property int index
                    readonly property bool ask: !!modelData.ask
                    width: list.width
                    height: 42
                    radius: Desk.r(8)
                    color: rowMouse.containsMouse || (start.query && index === list.currentIndex) ? Desk.raised : "transparent"
                    IconImage {
                        id: rowIcon
                        anchors { left: parent.left; leftMargin: 10; verticalCenter: parent.verticalCenter }
                        implicitSize: 26
                        source: row.ask ? "file:///usr/share/vibian/vibian-logo.png" : Desk.icon(row.modelData.icon)
                    }
                    Text {
                        anchors { left: rowIcon.right; leftMargin: 12; right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
                        text: row.ask ? "Ask the agent: “" + start.query.trim() + "”" : row.modelData.name
                        elide: Text.ElideRight
                        color: row.ask ? Desk.accent : Desk.fg
                        font.family: Desk.font
                        font.pixelSize: 14
                    }
                    MouseArea { id: rowMouse; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor
                                onClicked: start.choose(row.modelData) }
                }
            }

            Rectangle { Layout.fillWidth: true; Layout.preferredHeight: 1; color: Desk.line }
            RowLayout {
                Layout.fillWidth: true
                spacing: 6
                Chip { text: "Agent"; onClicked: { Desk.startOpen = false; Desk.ask("") } }
                Chip { text: "Lock"; onClicked: { Desk.startOpen = false; Desk.run(["loginctl", "lock-session"]) } }
                Item { Layout.fillWidth: true }
                Chip { text: start.confirm === "out" ? "Sure?" : "Log out"; danger: true
                       onClicked: start.power("out", ["hyprctl", "dispatch", "exit"]) }
                Chip { text: start.confirm === "reboot" ? "Sure?" : "Restart"; danger: true
                       onClicked: start.power("reboot", ["systemctl", "reboot"]) }
                Chip { text: start.confirm === "off" ? "Sure?" : "Shut down"; danger: true
                       onClicked: start.power("off", ["systemctl", "poweroff"]) }
            }
        }
    }
}
