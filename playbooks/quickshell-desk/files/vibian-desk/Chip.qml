// A small button.
import QtQuick

Rectangle {
    id: chip
    property string text: ""
    property bool danger: false
    property bool active: false
    signal clicked()

    implicitWidth: label.implicitWidth + 20
    implicitHeight: label.implicitHeight + 10
    radius: Desk.r(6)
    color: active ? Desk.accent : mouse.containsMouse ? Desk.line : Desk.raised
    Text {
        id: label
        anchors.centerIn: parent
        text: chip.text
        color: chip.active ? Desk.bg : chip.danger && mouse.containsMouse ? Desk.red : Desk.fg
        font.family: Desk.font
        font.pixelSize: 12
    }
    MouseArea {
        id: mouse
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: chip.clicked()
    }
}
