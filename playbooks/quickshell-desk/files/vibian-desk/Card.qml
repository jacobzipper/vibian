// A titled card, the frame of every desktop widget.
import QtQuick

Rectangle {
    id: card
    property string title: ""
    default property alias content: inner.data
    property alias headerRight: right.data

    color: Desk.card
    border.color: Desk.line
    border.width: 1
    radius: Desk.r(10)

    Rectangle { x: 0; y: 14; width: 3; height: 16; radius: 1; color: Desk.accent }
    Text {
        id: titleText
        anchors { top: parent.top; left: parent.left; topMargin: 12; leftMargin: 14 }
        text: card.title
        color: Desk.accent
        font.family: Desk.display
        font.pixelSize: 13
        font.weight: Font.DemiBold
        font.letterSpacing: 1.2
    }
    Row {
        id: right
        anchors { verticalCenter: titleText.verticalCenter; right: parent.right; rightMargin: 12 }
        spacing: 6
    }
    Item {
        id: inner
        anchors { top: titleText.bottom; left: parent.left; right: parent.right; bottom: parent.bottom
                  margins: 14; topMargin: 10 }
    }
}
