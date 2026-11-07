// CPU, memory and disk, read from /proc and df every 2 seconds.
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io

Widget {
    id: w
    name: "system"
    area: Place.system
    mask: Region {}

    property real cpu: 0
    property var last: null
    property real memUsed: 0
    property real memTotal: 1
    property real diskUsed: 0
    property real diskTotal: 1

    Process {
        id: probe
        command: ["sh", "-c", "head -n 1 /proc/stat; grep -E '^(MemTotal|MemAvailable):' /proc/meminfo; df -B1 --output=size,used / | tail -n 1"]
        stdout: StdioCollector {
            onStreamFinished: {
                const lines = text.trim().split("\n")
                if (lines.length < 4) return
                const c = lines[0].trim().split(/\s+/).slice(1).map(Number)
                const idle = c[3] + c[4], total = c.reduce((a, b) => a + b, 0)
                if (w.last) {
                    const dt = total - w.last.total
                    w.cpu = dt > 0 ? 1 - (idle - w.last.idle) / dt : 0
                }
                w.last = { idle: idle, total: total }
                const kb = l => Number(l.split(/\s+/)[1]) * 1024
                w.memTotal = kb(lines[1])
                w.memUsed = w.memTotal - kb(lines[2])
                const d = lines[3].trim().split(/\s+/).map(Number)
                w.diskTotal = d[0]; w.diskUsed = d[1]
            }
        }
    }
    Timer { interval: 2000; running: w.visible; repeat: true; triggeredOnStart: true; onTriggered: probe.running = true }

    function gb(b) { return (b / 1073741824).toFixed(b >= 107374182400 ? 0 : 1) + " GB" }

    component Meter: ColumnLayout {
        property string label
        property string value
        property real frac
        Layout.fillWidth: true
        spacing: 4
        RowLayout {
            Layout.fillWidth: true
            Text { text: label; color: Desk.dim; font.family: Desk.font; font.pixelSize: 12 }
            Item { Layout.fillWidth: true }
            Text { text: value; color: Desk.fg; font.family: Desk.font; font.pixelSize: 12 }
        }
        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 6
            radius: 3
            color: Desk.raised
            Rectangle {
                width: parent.width * Math.max(0, Math.min(1, frac))
                height: parent.height
                radius: 3
                color: frac >= 0.9 ? Desk.red : frac >= 0.75 ? Desk.yellow : Desk.accent
                Behavior on width { NumberAnimation { duration: 300 } }
            }
        }
    }

    Card {
        anchors.fill: parent
        title: "SYSTEM"
        ColumnLayout {
            anchors.fill: parent
            spacing: 10
            Meter { label: "Processor"; value: Math.round(w.cpu * 100) + "%"; frac: w.cpu }
            Meter { label: "Memory"; value: w.gb(w.memUsed) + " of " + w.gb(w.memTotal); frac: w.memUsed / w.memTotal }
            Meter { label: "Disk"; value: w.gb(w.diskUsed) + " of " + w.gb(w.diskTotal); frac: w.diskUsed / w.diskTotal }
            Item { Layout.fillHeight: true }
        }
    }
}
