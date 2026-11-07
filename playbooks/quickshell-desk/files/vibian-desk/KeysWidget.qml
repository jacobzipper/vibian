// The keys to know, from the Hyprland config the playbook wrote. Each row
// is the whole combination as keycaps: the modifier (desk.json's "mod"),
// then the keys.
import QtQuick
import QtQuick.Layouts
import Quickshell

Widget {
    name: "keys"
    area: Place.keys
    mask: Region {}

    // desk.json's mod is Hyprland's, like SUPER or SUPER+ALT
    readonly property var mods: Desk.mod.split("+").map(m => ({ SUPER: "Super", ALT: "Alt", CTRL: "Ctrl", CONTROL: "Ctrl", SHIFT: "Shift" })[m.toUpperCase()] || m)
    readonly property var keys: [
        [["Space"], "start menu"],
        [["A"], "the agent"],
        [["Enter"], "terminal"],
        [["Q"], "close the window"],
        [["F"], "fullscreen"],
        [["T"], "float or tile"],
        [["←↑↓→"], "move the focus"],
        [["Shift", "←↑↓→"], "move the window"],
        [["1–9"], "go to a workspace"],
        [["Shift", "1–9"], "move window there"],
        [["V"], "clipboard history"],
        [["Shift", "S"], "screenshot a region"],
        [["L"], "lock"],
        [["Shift", "E"], "log out"]
    ]

    component Cap: Rectangle {
        property string label
        implicitWidth: Math.max(22, capText.implicitWidth + 12)
        implicitHeight: 20
        radius: Desk.r(4)
        color: Desk.raised
        border.color: Desk.line
        // the key's lower edge, so it reads as a key
        Rectangle { anchors { left: parent.left; right: parent.right; bottom: parent.bottom } height: 2; radius: parent.radius; color: Desk.line }
        Text {
            id: capText
            anchors.centerIn: parent
            anchors.verticalCenterOffset: -1
            text: parent.label
            color: Desk.accent
            font.family: Desk.mono
            font.pixelSize: 11
        }
    }

    Card {
        anchors.fill: parent
        title: "KEYS"
        ColumnLayout {
            anchors.fill: parent
            spacing: 8
            Repeater {
                model: keys
                delegate: RowLayout {
                    id: row
                    required property var modelData
                    Layout.fillWidth: true
                    spacing: 10
                    Row {
                        Layout.preferredWidth: 164
                        spacing: 3
                        Repeater {
                            model: mods.concat(row.modelData[0])
                            delegate: Row {
                                required property string modelData
                                required property int index
                                spacing: 3
                                Text {
                                    visible: index > 0
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: "+"
                                    color: Desk.faint
                                    font.family: Desk.mono
                                    font.pixelSize: 10
                                }
                                Cap { label: modelData }
                            }
                        }
                    }
                    Text {
                        Layout.fillWidth: true
                        text: row.modelData[1]
                        elide: Text.ElideRight
                        color: Desk.dim
                        font.family: Desk.font
                        font.pixelSize: 12
                    }
                }
            }
            Item { Layout.fillHeight: true }
        }
    }
}
