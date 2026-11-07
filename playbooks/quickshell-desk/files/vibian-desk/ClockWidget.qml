// The big clock.
import QtQuick
import Quickshell

Widget {
    id: w
    name: "clock"
    area: Place.clock
    mask: Region {}   // clicks go through to the desktop

    SystemClock { id: clock; precision: SystemClock.Minutes }
    Column {
        anchors.centerIn: parent
        spacing: 0
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: Qt.formatTime(clock.date, "HH:mm")
            color: Desk.fg
            font.family: Desk.clockFont
            font.pixelSize: 64
            font.weight: Font.Light
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: Qt.formatDate(clock.date, "dddd d MMMM")
            color: Desk.accent
            font.family: Desk.font
            font.pixelSize: 15
        }
    }
}
