// A desktop widget: a window below all others on the first screen, at a
// fixed place. Hidden when desk.json's "widgets" leaves out its name.
import QtQuick
import Quickshell
import Quickshell.Wayland

PanelWindow {
    property string name: ""
    property rect area: Qt.rect(0, 0, 100, 100)
    screen: Desk.primary
    anchors { top: true; left: true }
    margins { top: area.y; left: area.x }
    implicitWidth: area.width
    implicitHeight: area.height
    exclusionMode: ExclusionMode.Ignore
    color: "transparent"
    visible: Desk.on(name) && area.width > 0
    WlrLayershell.namespace: "vibian-widget"
    WlrLayershell.layer: WlrLayer.Bottom
}
