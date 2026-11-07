// Ask the agent: type a request and press Return. It opens in a terminal
// (vibian agent run --prompt), so you can follow it and answer questions.
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Wayland

Widget {
    id: w
    name: "ask"
    area: Place.ask
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.OnDemand

    function send() {
        const t = input.text.trim()
        if (!t) return
        Desk.ask(t)
        input.text = ""
    }

    Card {
        anchors.fill: parent
        title: "ASK YOUR AGENT"
        ColumnLayout {
            anchors.fill: parent
            spacing: 10
            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 42
                radius: Desk.r(8)
                color: Desk.raised
                border.color: input.activeFocus ? Desk.accent : "transparent"
                TextInput {
                    id: input
                    anchors { fill: parent; leftMargin: 12; rightMargin: 12 }
                    verticalAlignment: TextInput.AlignVCenter
                    color: Desk.fg
                    selectionColor: Desk.accent
                    font.family: Desk.font
                    font.pixelSize: 14
                    clip: true
                    Keys.onReturnPressed: w.send()
                    Keys.onEnterPressed: w.send()
                    Keys.onEscapePressed: { text = ""; focus = false }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !input.text
                        text: "Set up my printer, make the text bigger, why is it slow…"
                        color: Desk.faint
                        font: input.font
                        elide: Text.ElideRight
                        width: parent.width
                    }
                }
            }
            RowLayout {
                spacing: 6
                Chip { text: "Ask"; active: true; onClicked: w.send() }
                Chip { text: "Open the agent"; onClicked: Desk.ask("") }
                Chip { text: "Check this computer"
                       onClicked: Desk.ask("Run vibian doctor and tell me what you find, and what you would fix.") }
            }
            Text {
                Layout.fillWidth: true
                text: Desk.mod + "+A opens the agent from anywhere. Everything it changes is listed under Recent changes, with a way back."
                wrapMode: Text.WordWrap
                color: Desk.faint
                font.family: Desk.font
                font.pixelSize: 11
            }
        }
    }
}
