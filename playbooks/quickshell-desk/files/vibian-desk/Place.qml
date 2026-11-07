// Where the desktop widgets go on the first screen: keys on the left, the
// clock and the agent in the middle, the system and recent changes on the
// right. The taskbar takes 44 px at the bottom.
pragma Singleton
import QtQuick
import Quickshell

Singleton {
    readonly property int sw: Desk.primary ? Desk.primary.width : 1920
    readonly property int sh: Desk.primary ? Desk.primary.height : 1080
    readonly property int gap: 24
    readonly property int side: 320
    readonly property int bottom: sh - 44 - gap
    readonly property int mid: Math.min(640, sw - 2 * (side + 2 * gap))
    readonly property int midX: (sw - mid) / 2
    // too narrow for three columns: only the middle one
    readonly property bool sides: mid >= 360
    readonly property int midW: sides ? mid : Math.min(640, sw - 2 * gap)
    readonly property int midLeft: sides ? midX : (sw - midW) / 2

    readonly property rect clock: Qt.rect(midLeft, 40, midW, 110)
    readonly property rect ask: Qt.rect(midLeft, 168, midW, 196)
    readonly property rect keys: sides ? Qt.rect(gap, gap, side, Math.min(456, bottom - gap)) : Qt.rect(0, 0, 0, 0)
    readonly property rect system: sides ? Qt.rect(sw - gap - side, gap, side, 164) : Qt.rect(0, 0, 0, 0)
    readonly property rect changes: sides ? Qt.rect(sw - gap - side, gap + 164 + 16, side, Math.min(312, bottom - gap - 180)) : Qt.rect(0, 0, 0, 0)
}
