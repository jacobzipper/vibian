// Stocks: each ticker with today's sparkline, price and change, from
// stocks.py (Yahoo Finance, no key) every minute. The tickers are in
// tickers.json here; "+ add" adds one, hover a row and click × to remove it.
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import Quickshell.Wayland

Widget {
    id: w
    name: "stocks"
    area: Place.stocks
    WlrLayershell.keyboardFocus: adding ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None

    property var tickers: []
    property var quotes: ({})
    property int updated: 0
    property bool adding: false

    FileView {
        id: list
        path: Quickshell.shellDir + "/tickers.json"
        watchChanges: true
        onFileChanged: reload()
        onLoaded: {
            try { w.tickers = JSON.parse(text()).tickers || [] } catch (e) { console.warn("tickers.json:", e) }
            fetch.running = true
        }
    }
    function save(l) {
        tickers = l
        list.setText(JSON.stringify({ tickers: l }, null, 2) + "\n")
        fetch.running = true
    }
    function add(sym) {
        sym = sym.trim().toUpperCase()
        if (/^[A-Z0-9.^=-]{1,15}$/.test(sym) && tickers.indexOf(sym) < 0) save(tickers.concat([sym]))
    }

    Process {
        id: fetch
        command: ["python3", "-I", Quickshell.shellDir + "/stocks.py"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const j = JSON.parse(text)
                    const q = {}
                    j.quotes.forEach(x => q[x.symbol] = x)
                    w.quotes = q
                    w.updated = j.time
                } catch (e) {}
            }
        }
    }
    Timer { interval: 60000; running: w.visible; repeat: true; onTriggered: fetch.running = true }

    function money(v) {
        if (v === undefined || v === null) return "—"
        const a = Math.abs(v)
        return v.toFixed(a >= 1000 ? 0 : a >= 1 ? 2 : 4)
    }

    Card {
        anchors.fill: parent
        title: "STOCKS"
        headerRight: [
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: w.updated ? Qt.formatTime(new Date(w.updated * 1000), "HH:mm") : ""
                color: Desk.faint
                font.family: Desk.mono
                font.pixelSize: 10
            },
            Chip {
                text: w.adding ? "done" : "+ add"
                active: w.adding
                onClicked: { w.adding = !w.adding; if (w.adding) addInput.forceActiveFocus() }
            }
        ]

        Flickable {
            anchors.fill: parent
            contentHeight: rows.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ColumnLayout {
                id: rows
                width: parent.width
                spacing: 4

                Rectangle {
                    visible: w.adding
                    Layout.fillWidth: true
                    Layout.preferredHeight: 32
                    radius: Desk.r(6)
                    color: Desk.raised
                    border.color: Desk.accent
                    TextInput {
                        id: addInput
                        anchors { fill: parent; leftMargin: 10; rightMargin: 10 }
                        verticalAlignment: TextInput.AlignVCenter
                        color: Desk.fg
                        selectionColor: Desk.accent
                        font.family: Desk.mono
                        font.pixelSize: 13
                        font.capitalization: Font.AllUppercase
                        onAccepted: { w.add(text); text = "" }
                        Keys.onEscapePressed: w.adding = false
                        Text {
                            visible: !addInput.text
                            anchors.verticalCenter: parent.verticalCenter
                            text: "a ticker, then Enter: AAPL, BTC-USD, ^GSPC"
                            color: Desk.faint
                            font.family: Desk.mono
                            font.pixelSize: 12
                        }
                    }
                }

                Text {
                    visible: !w.tickers.length
                    text: "No tickers yet: press + add."
                    color: Desk.faint
                    font.family: Desk.font
                    font.pixelSize: 12
                }

                Repeater {
                    model: w.tickers
                    delegate: Rectangle {
                        id: row
                        required property string modelData
                        readonly property var q: w.quotes[modelData] || ({})
                        readonly property bool up: (q.change || 0) >= 0
                        readonly property color tone: q.error ? Desk.faint : up ? Desk.green : Desk.red
                        Layout.fillWidth: true
                        Layout.preferredHeight: 40
                        radius: Desk.r(6)
                        color: hover.hovered ? Desk.raised : "transparent"
                        HoverHandler { id: hover }

                        Column {
                            anchors { left: parent.left; leftMargin: 8; verticalCenter: parent.verticalCenter }
                            width: 96
                            Text { text: row.modelData; color: Desk.fg; font.family: Desk.mono; font.pixelSize: 13; font.bold: true }
                            Text {
                                width: parent.width
                                text: row.q.error ? "no data" : row.q.name || ""
                                elide: Text.ElideRight
                                color: Desk.faint
                                font.family: Desk.font
                                font.pixelSize: 9
                            }
                        }

                        Canvas {
                            id: spark
                            x: 112
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.max(40, row.width - 112 - 130)
                            height: 28
                            property var pts: row.q.points || []
                            property real prev: row.q.prev || 0
                            onPtsChanged: requestPaint()
                            onWidthChanged: requestPaint()
                            onPaint: {
                                const ctx = getContext("2d")
                                ctx.reset()
                                if (pts.length < 2) return
                                let lo = Math.min.apply(null, pts), hi = Math.max.apply(null, pts)
                                if (prev) { lo = Math.min(lo, prev); hi = Math.max(hi, prev) }
                                const span = (hi - lo) || 1
                                const y = v => height - 2 - (v - lo) / span * (height - 4)
                                if (prev) {   // yesterday's close, dotted
                                    ctx.strokeStyle = Qt.rgba(Desk.faint.r, Desk.faint.g, Desk.faint.b, 0.6)
                                    ctx.lineWidth = 1
                                    for (let x = 0; x < width; x += 6) { ctx.beginPath(); ctx.moveTo(x, y(prev)); ctx.lineTo(x + 2, y(prev)); ctx.stroke() }
                                }
                                ctx.beginPath()
                                pts.forEach((v, i) => {
                                    const x = i / (pts.length - 1) * width
                                    i ? ctx.lineTo(x, y(v)) : ctx.moveTo(x, y(v))
                                })
                                ctx.strokeStyle = row.tone
                                ctx.lineWidth = 1.6
                                ctx.stroke()
                                ctx.lineTo(width, height); ctx.lineTo(0, height); ctx.closePath()
                                ctx.fillStyle = Qt.rgba(row.tone.r, row.tone.g, row.tone.b, 0.12)
                                ctx.fill()
                            }
                        }

                        Column {
                            anchors { right: parent.right; rightMargin: hover.hovered ? 30 : 8; verticalCenter: parent.verticalCenter }
                            Text {
                                anchors.right: parent.right
                                text: w.money(row.q.price)
                                color: Desk.fg
                                font.family: Desk.mono
                                font.pixelSize: 13
                            }
                            Text {
                                anchors.right: parent.right
                                text: row.q.error || row.q.change === undefined ? ""
                                      : (row.up ? "+" : "") + w.money(row.q.change) + "  " + (row.up ? "+" : "") + row.q.pct.toFixed(2) + "%"
                                color: row.tone
                                font.family: Desk.mono
                                font.pixelSize: 11
                            }
                        }
                        Text {
                            visible: hover.hovered
                            anchors { right: parent.right; rightMargin: 8; verticalCenter: parent.verticalCenter }
                            text: "×"
                            color: del.containsMouse ? Desk.red : Desk.dim
                            font.family: Desk.mono
                            font.pixelSize: 18
                            MouseArea {
                                id: del
                                anchors { fill: parent; margins: -6 }
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: w.save(w.tickers.filter(t => t !== row.modelData))
                            }
                        }
                    }
                }
            }
        }
    }
}
