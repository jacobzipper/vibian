// What changed on this computer, newest first, from `vibian history`. Each
// change can be undone: click one to ask the agent about it.
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io

Widget {
    id: w
    name: "changes"
    area: Place.changes

    property var changes: []

    Process {
        id: history
        command: ["vibian", "history", "--json", "-n", "8"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { w.changes = (JSON.parse(text) || []).slice().reverse() } catch (e) { w.changes = [] }
            }
        }
    }
    Timer { interval: 60000; running: w.visible; repeat: true; triggeredOnStart: true; onTriggered: history.running = true }

    function label(c) {
        const tools = { apt: "Packages installed or removed", dpkg: "Packages installed or removed", "agent turn": "Agent turn" }
        return (tools[c.what] || c.what) + (c.agent ? " (" + c.agent + ")" : "")
    }

    function ago(t) {
        const s = (Date.now() - new Date(t).getTime()) / 1000
        if (s < 90) return "just now"
        if (s < 5400) return Math.round(s / 60) + " min ago"
        if (s < 129600) return Math.round(s / 3600) + " h ago"
        return Math.round(s / 86400) + " days ago"
    }

    Card {
        anchors.fill: parent
        title: "RECENT CHANGES"
        headerRight: [
            Chip { text: "refresh"; onClicked: history.running = true }
        ]
        ColumnLayout {
            id: list
            anchors.fill: parent
            spacing: 4
            Text {
                visible: !w.changes.length
                Layout.fillWidth: true
                text: "Nothing yet. Whatever the agent changes on this computer shows up here, with a snapshot to go back to."
                wrapMode: Text.WordWrap
                color: Desk.faint
                font.family: Desk.font
                font.pixelSize: 12
            }
            Repeater {
                // as many as fit
                model: w.changes.slice(0, Math.max(0, Math.floor((list.height + 4) / 44)))
                delegate: Rectangle {
                    id: change
                    required property var modelData
                    Layout.fillWidth: true
                    Layout.preferredHeight: 40
                    radius: Desk.r(6)
                    color: changeMouse.containsMouse ? Desk.raised : "transparent"
                    Column {
                        anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; leftMargin: 6; rightMargin: 6 }
                        Text {
                            width: parent.width
                            text: w.label(change.modelData)
                            elide: Text.ElideRight
                            color: Desk.fg
                            font.family: Desk.font
                            font.pixelSize: 12
                        }
                        Text {
                            width: parent.width
                            text: w.ago(change.modelData.time) + (change.modelData.ledger ? "  ·  " + change.modelData.ledger : "")
                            elide: Text.ElideRight
                            color: Desk.faint
                            font.family: Desk.font
                            font.pixelSize: 10
                        }
                    }
                    MouseArea {
                        id: changeMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: Desk.ask("Tell me about this change from vibian history, and how I would undo it: "
                                            + (change.modelData.ledger || "") + " " + change.modelData.what)
                    }
                }
            }
            Item { Layout.fillHeight: true }
        }
    }
}
