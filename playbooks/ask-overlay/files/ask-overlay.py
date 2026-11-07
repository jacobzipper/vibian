#!/usr/bin/python3
"""Vibian's Ask overlay: a box to type a request for the agent, and the
recent requests with their answers. Requests go to `vibian ask`, which runs
them in the background and sends a notification when each is done.

A layer-shell overlay on Wayland desktops that have it (Hyprland, Sway,
niri, KDE, COSMIC); a plain window on GNOME and X11. One instance: running it
again while it is open closes it.

Options: --accent RRGGBB, --bg RRGGBB.
"""
import argparse
import json
import subprocess
import sys

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
from gi.repository import Gdk, Gio, GLib, Gtk, Pango  # noqa: E402

try:
    gi.require_version("Gtk4LayerShell", "1.0")
    from gi.repository import Gtk4LayerShell as LayerShell  # noqa: E402
except (ValueError, ImportError):
    LayerShell = None

APP_ID = "dev.vibian.Ask"
TITLE = "Ask your agent"
STATE_MARK = {"running": "…", "done": "✓", "failed": "✗", "stopped": "■"}


def layer_shell():
    """Whether this session can show a layer-shell overlay."""
    try:
        return LayerShell is not None and LayerShell.is_supported()
    except GLib.Error:  # the bindings without the library
        return False


def vibian(*args):
    """Run the vibian CLI; returns (ok, stdout, stderr)."""
    try:
        p = subprocess.run(["vibian", *args], capture_output=True, text=True, timeout=20)
        return p.returncode == 0, p.stdout, p.stderr
    except (OSError, subprocess.TimeoutExpired) as e:
        return False, "", str(e)


def css(accent, bg):
    return f"""
window.vibian-ask {{ background: transparent; }}
.card {{
  background: alpha(#{bg}, 0.97); color: #c0caf5;
  border: 2px solid #{accent}; border-radius: 14px; padding: 18px;
}}
.title {{ font-size: 15px; font-weight: bold; color: #{accent}; }}
.hint {{ font-size: 12px; color: #737aa2; }}
entry {{
  font-size: 17px; padding: 10px; border-radius: 10px;
  background: alpha(#ffffff, 0.06); color: #c0caf5; border: 1px solid alpha(#{accent}, 0.5);
}}
.row {{ padding: 6px 8px; border-radius: 8px; }}
.row:hover {{ background: alpha(#{accent}, 0.12); }}
.mark {{ font-weight: bold; color: #{accent}; min-width: 18px; }}
.failed .mark {{ color: #f7768e; }}
.answer {{
  font-family: monospace; font-size: 12px; padding: 10px; border-radius: 8px;
  background: alpha(#000000, 0.25);
}}
button {{
  padding: 2px 10px; border-radius: 8px; border: none; box-shadow: none;
  background: alpha(#{accent}, 0.18); color: #c0caf5;
}}
button:hover {{ background: alpha(#{accent}, 0.32); }}
"""


class Overlay(Gtk.ApplicationWindow):
    def __init__(self, app, opts):
        super().__init__(application=app, title=TITLE)
        self.add_css_class("vibian-ask")
        self.set_default_size(680, -1)
        self.set_resizable(False)
        self.open_id = None
        self.asks = []

        if layer_shell():
            print("vibian-ask: an overlay (layer shell)", file=sys.stderr)
            LayerShell.init_for_window(self)
            LayerShell.set_namespace(self, "vibian-ask")
            LayerShell.set_layer(self, LayerShell.Layer.OVERLAY)
            LayerShell.set_keyboard_mode(self, LayerShell.KeyboardMode.EXCLUSIVE)
            LayerShell.set_anchor(self, LayerShell.Edge.TOP, True)
            LayerShell.set_margin(self, LayerShell.Edge.TOP, 160)
        else:
            print("vibian-ask: a window (no layer shell here)", file=sys.stderr)
            self.set_decorated(False)

        card = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        card.add_css_class("card")
        self.set_child(card)

        head = Gtk.Box(spacing=8)
        title = Gtk.Label(label=TITLE, xalign=0, hexpand=True)
        title.add_css_class("title")
        hint = Gtk.Label(label="Return sends · Esc closes")
        hint.add_css_class("hint")
        head.append(title)
        head.append(hint)
        card.append(head)

        self.entry = Gtk.Entry(placeholder_text="Make the text bigger, install Steam, why is the fan loud…")
        self.entry.connect("activate", self.send)
        card.append(self.entry)

        self.status = Gtk.Label(xalign=0, wrap=True, visible=False)
        self.status.add_css_class("hint")
        card.append(self.status)

        self.list = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        card.append(self.list)

        self.answer = Gtk.Label(xalign=0, yalign=0, wrap=True, selectable=True)
        self.answer.set_wrap_mode(Pango.WrapMode.WORD_CHAR)
        self.answer.add_css_class("answer")
        self.scroll = Gtk.ScrolledWindow(max_content_height=320, propagate_natural_height=True)
        self.scroll.set_policy(Gtk.PolicyType.NEVER, Gtk.PolicyType.AUTOMATIC)
        self.scroll.set_child(self.answer)
        self.scroll.set_visible(False)
        card.append(self.scroll)

        keys = Gtk.EventControllerKey()
        keys.connect("key-pressed", self.on_key)
        self.add_controller(keys)

        self.refresh()
        GLib.timeout_add_seconds(2, self.refresh)

    def on_key(self, _ctl, keyval, _code, _state):
        if keyval == Gdk.KEY_Escape:
            self.get_application().quit()
            return True
        return False

    def send(self, _entry):
        text = self.entry.get_text().strip()
        if not text:
            return
        ok, out, err = vibian("ask", text)
        if ok:
            self.entry.set_text("")
            self.say("Sent. You'll get a notification when it's done.")
            self.open_id = out.strip()
        else:
            msg = (err or out).strip() or "vibian ask failed"
            print(f"vibian-ask: {msg}", file=sys.stderr)
            self.say(msg)
        self.refresh()

    def say(self, text):
        self.status.set_text(text)
        self.status.set_visible(bool(text))

    def refresh(self):
        ok, out, _ = vibian("asks", "--json", "-n", "6")
        try:
            asks = json.loads(out) if ok else []
        except ValueError:
            asks = []
        if [(a["id"], a["state"], a.get("answer")) for a in asks] != [
            (a["id"], a["state"], a.get("answer")) for a in self.asks
        ]:
            self.asks = asks
            self.draw()
        return True

    def draw(self):
        while (child := self.list.get_first_child()) is not None:
            self.list.remove(child)
        for a in self.asks:
            row = Gtk.Box(spacing=8)
            row.add_css_class("row")
            row.add_css_class(a["state"])
            mark = Gtk.Label(label=STATE_MARK.get(a["state"], "?"))
            mark.add_css_class("mark")
            req = Gtk.Label(label=a["request"], xalign=0, hexpand=True, ellipsize=Pango.EllipsizeMode.END)
            row.append(mark)
            row.append(req)
            if a["state"] == "running":
                stop = Gtk.Button(label="Stop")
                stop.connect("clicked", lambda _b, i=a["id"]: (vibian("asks", "stop", i), self.refresh()))
                row.append(stop)
            click = Gtk.GestureClick()
            click.connect("released", lambda *_x, i=a["id"]: self.toggle(i))
            row.add_controller(click)
            self.list.append(row)
        self.show_answer()

    def toggle(self, ask_id):
        self.open_id = None if self.open_id == ask_id else ask_id
        self.show_answer()

    def show_answer(self):
        a = next((a for a in self.asks if a["id"] == self.open_id), None)
        if a is None:
            self.scroll.set_visible(False)
            return
        text = a.get("answer") or ("Working on it…" if a["state"] == "running" else "(no answer)")
        if a.get("ledger"):
            text += f"\n\nIt changed files; vibian recorded it as {a['ledger']}."
        self.answer.set_text(text)
        self.scroll.set_visible(True)


class App(Gtk.Application):
    def __init__(self, opts):
        super().__init__(application_id=APP_ID, flags=Gio.ApplicationFlags.HANDLES_COMMAND_LINE)
        self.opts = opts
        self.win = None

    def do_command_line(self, _cmdline):
        if self.win is not None:
            # a second launch (the hotkey again) closes it
            self.quit()
            return 0
        # dark widgets under the card's colours
        Gtk.Settings.get_default().props.gtk_application_prefer_dark_theme = True
        provider = Gtk.CssProvider()
        provider.load_from_string(css(self.opts.accent, self.opts.bg))
        Gtk.StyleContext.add_provider_for_display(
            Gdk.Display.get_default(), provider, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION
        )
        self.win = Overlay(self, self.opts)
        self.win.present()
        self.win.entry.grab_focus()
        return 0


def main():
    ap = argparse.ArgumentParser(description="Vibian's Ask overlay")
    ap.add_argument("--accent", default="7aa2f7")
    ap.add_argument("--bg", default="16161e")
    opts = ap.parse_args()
    for v in (opts.accent, opts.bg):
        if len(v) != 6 or any(c not in "0123456789abcdefABCDEF" for c in v):
            sys.exit("colours are 6 hex digits")
    GLib.set_prgname("vibian-ask")
    return App(opts).run([sys.argv[0]])


if __name__ == "__main__":
    sys.exit(main())
