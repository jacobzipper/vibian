// VPN: Mullvad's state and location, connect or disconnect, and a country
// to connect through. Talks to the Mullvad app's CLI (`mullvad status -j
// listen` for live state) and never reads the account. Hidden until the
// Mullvad app is installed: ask the agent to set it up.
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io

Widget {
    id: w
    name: "vpn"
    area: Place.vpn
    visible: Desk.on(name) && area.width > 0 && have

    property bool have: false
    property var st: ({})            // mullvad status -j
    property string relay: ""        // the location constraint, like "city dal, us"
    property var countries: []       // [{name, code}]
    property bool picking: false
    readonly property string vpnState: st.state || ""
    readonly property var loc: (st.details && st.details.location) || ({})

    Process {
        command: ["sh", "-c", "command -v mullvad"]
        running: true
        onExited: code => { w.have = code === 0; if (w.have) { status.running = true; live.running = true; relayGet.running = true } }
    }
    Process {
        id: status
        command: ["mullvad", "status", "-j"]
        stdout: StdioCollector { onStreamFinished: w.parse(text) }
    }
    // one JSON object per line on every change; restarted if the daemon goes away
    Process {
        id: live
        command: ["mullvad", "status", "-j", "listen"]
        stdout: SplitParser { onRead: line => w.parse(line) }
    }
    Timer { interval: 10000; running: w.have && !live.running; repeat: true; onTriggered: { status.running = true; live.running = true } }
    function parse(t) { try { const j = JSON.parse(t); if (j.state) w.st = j } catch (e) {} }

    Process {
        id: relayGet
        command: ["mullvad", "relay", "get"]
        stdout: StdioCollector {
            onStreamFinished: { const m = /Location:\s*(.+)/.exec(text); w.relay = m ? m[1].trim() : "" }
        }
    }
    Process {
        id: relayList
        command: ["mullvad", "relay", "list"]
        stdout: StdioCollector {
            onStreamFinished: {
                const out = []
                const re = /^(\S[^\n(]*) \(([a-z]{2})\)$/gm
                let m
                while ((m = re.exec(text))) out.push({ name: m[1], code: m[2] })
                w.countries = out
            }
        }
    }
    Process {
        id: act
        onExited: relayGet.running = true
    }
    function run(args) { if (!act.running) { act.command = ["mullvad"].concat(args); act.running = true } }
    function pick(code) { run(["relay", "set", "location", code]); picking = false }

    readonly property color tone: vpnState === "connected" ? Desk.green
                                  : vpnState === "connecting" || vpnState === "disconnecting" ? Desk.yellow
                                  : vpnState === "error" ? Desk.red : Desk.faint
    function said() {
        switch (vpnState) {
        case "connected": return "Connected"
        case "connecting": return "Connecting…"
        case "disconnecting": return "Disconnecting…"
        case "error": return "Blocked: Mullvad hit an error"
        case "disconnected": return "Not connected"
        default: return "Mullvad isn't running"
        }
    }

    Card {
        anchors.fill: parent
        title: "VPN"
        headerRight: [
            Chip {
                text: w.picking ? "back" : "country"
                active: w.picking
                onClicked: { w.picking = !w.picking; if (w.picking && !w.countries.length) relayList.running = true }
            }
        ]

        ColumnLayout {
            anchors.fill: parent
            visible: !w.picking
            spacing: 6
            RowLayout {
                spacing: 8
                Rectangle { implicitWidth: 10; implicitHeight: 10; radius: 5; color: w.tone }
                Text { text: w.said(); color: Desk.fg; font.family: Desk.display; font.pixelSize: 16 }
            }
            Text {
                Layout.fillWidth: true
                visible: !!w.loc.country
                text: (w.loc.city ? w.loc.city + ", " : "") + (w.loc.country || "")
                elide: Text.ElideRight
                color: Desk.dim
                font.family: Desk.font
                font.pixelSize: 12
            }
            Text {
                Layout.fillWidth: true
                text: (w.loc.hostname || "") + (w.relay ? (w.loc.hostname ? "  ·  " : "") + "set to " + w.relay : "")
                elide: Text.ElideRight
                color: Desk.faint
                font.family: Desk.mono
                font.pixelSize: 10
            }
            RowLayout {
                Layout.topMargin: 6
                spacing: 6
                Chip {
                    text: w.vpnState === "connected" || w.vpnState === "connecting" ? "Disconnect" : "Connect"
                    active: !(w.vpnState === "connected" || w.vpnState === "connecting")
                    onClicked: w.run([w.vpnState === "connected" || w.vpnState === "connecting" ? "disconnect" : "connect"])
                }
                Chip { text: "Reconnect"; visible: w.vpnState === "connected"; onClicked: w.run(["reconnect"]) }
            }
            Item { Layout.fillHeight: true }
        }

        ListView {
            anchors.fill: parent
            visible: w.picking
            clip: true
            spacing: 2
            boundsBehavior: Flickable.StopAtBounds
            model: w.countries
            header: Text {
                visible: !w.countries.length
                height: visible ? implicitHeight : 0
                text: "Loading the server list…"
                color: Desk.faint
                font.family: Desk.font
                font.pixelSize: 12
            }
            delegate: Rectangle {
                id: c
                required property var modelData
                width: ListView.view.width
                height: 24
                radius: Desk.r(5)
                color: m.containsMouse ? Desk.raised : "transparent"
                Text {
                    anchors { left: parent.left; leftMargin: 8; verticalCenter: parent.verticalCenter }
                    text: c.modelData.name
                    color: w.relay.endsWith(" " + c.modelData.code) || w.relay === "country " + c.modelData.code ? Desk.accent : Desk.fg
                    font.family: Desk.font
                    font.pixelSize: 12
                }
                Text {
                    anchors { right: parent.right; rightMargin: 8; verticalCenter: parent.verticalCenter }
                    text: c.modelData.code
                    color: Desk.faint
                    font.family: Desk.mono
                    font.pixelSize: 10
                }
                MouseArea { id: m; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: w.pick(c.modelData.code) }
            }
        }
    }
}
