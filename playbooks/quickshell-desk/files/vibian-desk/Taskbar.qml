// The taskbar, on every screen: start button, workspaces, this workspace's
// windows, tray, battery, volume and clock.
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Hyprland
import Quickshell.Wayland
import Quickshell.Widgets
import Quickshell.Services.SystemTray
import Quickshell.Services.UPower
import Quickshell.Services.Pipewire

PanelWindow {
    id: bar
    required property var modelData
    screen: modelData
    anchors { bottom: true; left: true; right: true }
    implicitHeight: 44
    color: "transparent"
    WlrLayershell.namespace: "vibian-taskbar"
    // bottom, not top: keeps its reserved strip but a fullscreen window covers it
    WlrLayershell.layer: WlrLayer.Bottom

    readonly property var monitor: Hyprland.monitorFor(screen)
    readonly property var workspaces: Hyprland.workspaces.values
        .filter(w => w.id > 0 && w.monitor === monitor).sort((a, b) => a.id - b.id)
    readonly property var windows: Hyprland.toplevels.values
        .filter(t => monitor && t.workspace && t.workspace === monitor.activeWorkspace)

    function addr(t) { const a = String(t.address); return a.startsWith("0x") ? a : "0x" + a }
    function appId(t) {
        if (t.wayland && t.wayland.appId) return t.wayland.appId
        const o = t.lastIpcObject
        return o && o["class"] ? o["class"] : ""
    }

    Component.onCompleted: Hyprland.refreshToplevels()
    Connections {
        target: Hyprland
        function onRawEvent(ev) {
            if (["openwindow", "closewindow", "movewindow", "movewindowv2", "windowtitlev2"].indexOf(ev.name) >= 0)
                Hyprland.refreshToplevels()
        }
    }

    SystemClock { id: clock; precision: SystemClock.Minutes }
    PwObjectTracker { objects: Pipewire.defaultAudioSink ? [Pipewire.defaultAudioSink] : [] }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(Desk.bg.r, Desk.bg.g, Desk.bg.b, 0.96)
        Rectangle { anchors { top: parent.top; left: parent.left; right: parent.right } height: 1; color: Desk.line }
    }

    RowLayout {
        anchors { left: parent.left; right: tray.left; verticalCenter: parent.verticalCenter; leftMargin: 6; rightMargin: 8 }
        spacing: 4

        // start
        Rectangle {
            Layout.preferredWidth: 40
            Layout.preferredHeight: 36
            radius: Desk.r(8)
            color: Desk.startOpen ? Desk.accent : startMouse.containsMouse ? Desk.raised : "transparent"
            Image {
                anchors.centerIn: parent
                width: 26; height: 26
                source: "file:///usr/share/vibian/vibian-logo.png"
                fillMode: Image.PreserveAspectFit
                smooth: true
                mipmap: true
            }
            MouseArea { id: startMouse; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor
                        onClicked: Desk.startOpen = !Desk.startOpen }
        }

        // workspaces
        Repeater {
            model: bar.workspaces
            delegate: Rectangle {
                required property var modelData
                readonly property bool current: bar.monitor && bar.monitor.activeWorkspace === modelData
                Layout.preferredWidth: 28
                Layout.preferredHeight: 28
                radius: Desk.r(6)
                color: current ? Desk.accent : wsMouse.containsMouse ? Desk.raised : "transparent"
                border.color: modelData.urgent ? Desk.red : "transparent"
                Text {
                    anchors.centerIn: parent
                    text: parent.modelData.name
                    color: parent.current ? Desk.bg : Desk.dim
                    font.family: Desk.font
                    font.pixelSize: 13
                    font.weight: Font.DemiBold
                }
                MouseArea { id: wsMouse; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor
                            onClicked: Hyprland.dispatch("workspace " + parent.modelData.id) }
            }
        }

        Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 24; Layout.leftMargin: 4; Layout.rightMargin: 4; color: Desk.line }

        // windows
        Repeater {
            model: bar.windows
            delegate: Rectangle {
                id: task
                required property var modelData
                readonly property var entry: Desk.entryFor(bar.appId(modelData))
                Layout.preferredWidth: Math.min(200, Math.max(44, (bar.width - 520) / Math.max(1, bar.windows.length)))
                Layout.preferredHeight: 36
                radius: Desk.r(8)
                color: modelData.activated ? Desk.raised : taskMouse.containsMouse ? Qt.darker(Desk.raised, 1.2) : "transparent"
                Rectangle {
                    visible: task.modelData.activated
                    anchors { bottom: parent.bottom; horizontalCenter: parent.horizontalCenter }
                    width: parent.width - 16; height: 2; radius: 1; color: Desk.accent
                }
                IconImage {
                    id: taskIcon
                    anchors { left: parent.left; leftMargin: 9; verticalCenter: parent.verticalCenter }
                    implicitSize: 22
                    source: Desk.icon(task.entry ? task.entry.icon : bar.appId(task.modelData))
                }
                Text {
                    anchors { left: taskIcon.right; leftMargin: 8; right: parent.right; rightMargin: 8; verticalCenter: parent.verticalCenter }
                    visible: parent.width > 80
                    text: task.modelData.title || (task.entry ? task.entry.name : "")
                    elide: Text.ElideRight
                    color: task.modelData.activated ? Desk.fg : Desk.dim
                    font.family: Desk.font
                    font.pixelSize: 12
                }
                MouseArea {
                    id: taskMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    acceptedButtons: Qt.LeftButton | Qt.MiddleButton
                    cursorShape: Qt.PointingHandCursor
                    onClicked: (ev) => Hyprland.dispatch((ev.button === Qt.MiddleButton ? "closewindow address:" : "focuswindow address:") + bar.addr(task.modelData))
                }
            }
        }
        Item { Layout.fillWidth: true }
    }

    RowLayout {
        id: tray
        anchors { right: parent.right; verticalCenter: parent.verticalCenter; rightMargin: 12 }
        spacing: 10

        Repeater {
            model: SystemTray.items
            delegate: MouseArea {
                id: trayItem
                required property var modelData
                Layout.preferredWidth: 20
                Layout.preferredHeight: 20
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                cursorShape: Qt.PointingHandCursor
                IconImage { anchors.fill: parent; source: trayItem.modelData.icon }
                onClicked: (ev) => {
                    if (ev.button === Qt.LeftButton && !modelData.onlyMenu) modelData.activate()
                    else if (modelData.hasMenu) modelData.display(bar, mapToItem(null, 0, 0).x, 0)
                }
            }
        }

        // volume: scroll to change, click to mute
        Text {
            readonly property var sink: Pipewire.defaultAudioSink
            visible: !!(sink && sink.audio)
            text: !visible ? "" : sink.audio.muted ? "muted" : "vol " + Math.round(sink.audio.volume * 100) + "%"
            color: Desk.dim
            font.family: Desk.font
            font.pixelSize: 12
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: parent.sink.audio.muted = !parent.sink.audio.muted
                onWheel: (ev) => {
                    const a = parent.sink.audio
                    a.volume = Math.max(0, Math.min(1, a.volume + (ev.angleDelta.y > 0 ? 0.05 : -0.05)))
                }
            }
        }

        // battery, on laptops
        Text {
            readonly property var dev: UPower.displayDevice
            readonly property int pct: dev ? Math.round(dev.percentage <= 1 ? dev.percentage * 100 : dev.percentage) : 0
            visible: !!(dev && dev.isLaptopBattery)
            text: (dev && dev.state === UPowerDeviceState.Charging ? "charging " : "bat ") + pct + "%"
            color: pct <= 15 ? Desk.red : Desk.dim
            font.family: Desk.font
            font.pixelSize: 12
        }

        Column {
            Text { anchors.right: parent.right; text: Qt.formatTime(clock.date, "HH:mm"); color: Desk.fg
                   font.family: Desk.font; font.pixelSize: 13; font.weight: Font.DemiBold }
            Text { anchors.right: parent.right; text: Qt.formatDate(clock.date, "ddd d MMM"); color: Desk.faint
                   font.family: Desk.font; font.pixelSize: 10 }
        }
    }
}
