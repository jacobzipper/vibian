// The keys to know, from the Hyprland config the playbook wrote.
import QtQuick
import QtQuick.Layouts
import Quickshell

Widget {
    name: "keys"
    area: Place.keys
    mask: Region {}

    readonly property var keys: [
        ["Space", "start menu"],
        ["A", "the agent"],
        ["Return", "terminal"],
        ["Q", "close the window"],
        ["F", "fullscreen"],
        ["T", "float or tile the window"],
        ["arrows", "move the focus"],
        ["Shift+arrows", "move the window"],
        ["1 … 9", "go to a workspace"],
        ["Shift+1 … 9", "send the window there"],
        ["V", "clipboard history"],
        ["Shift+S", "screenshot a region"],
        ["L", "lock"],
        ["Shift+E", "log out"]
    ]

    Card {
        anchors.fill: parent
        title: "KEYS"
        headerRight: [
            Text {
                text: "hold " + Desk.mod
                color: Desk.faint
                font.family: Desk.mono
                font.pixelSize: 11
            }
        ]
        ColumnLayout {
            anchors.fill: parent
            spacing: 7
            Repeater {
                model: keys
                delegate: RowLayout {
                    required property var modelData
                    Layout.fillWidth: true
                    spacing: 10
                    Rectangle {
                        Layout.preferredWidth: 104
                        Layout.preferredHeight: 22
                        radius: Desk.r(5)
                        color: Desk.raised
                        Text {
                            anchors.centerIn: parent
                            text: parent.parent.modelData[0]
                            color: Desk.accent
                            font.family: Desk.mono
                            font.pixelSize: 11
                        }
                    }
                    Text {
                        Layout.fillWidth: true
                        text: parent.modelData[1]
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
