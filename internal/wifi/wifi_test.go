package wifi

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	out := ` :42:WPA2:Cafe\:Guest
*:80:WPA2:home
 :35:WPA2:home
 :60:--:open net
 :90:WPA2:
 :70:WPA1 WPA2:back\\slash`
	want := []Network{
		{SSID: "home", Signal: 80, Security: "WPA2", InUse: true},
		{SSID: `back\slash`, Signal: 70, Security: "WPA1 WPA2"},
		{SSID: "open net", Signal: 60},
		{SSID: "Cafe:Guest", Signal: 42, Security: "WPA2"},
	}
	if got := Parse(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse:\n got %+v\nwant %+v", got, want)
	}
	if got := Parse(""); len(got) != 0 {
		t.Fatalf("Parse(\"\") = %+v", got)
	}
}

func TestPickDevice(t *testing.T) {
	for _, c := range []struct{ out, want string }{
		{"enp0s2:ethernet:connected\nwlan1:wifi:connected\nwlan0:wifi:disconnected\np2p-dev-wlan0:wifi-p2p:disconnected", "wlan0"},
		{"wlp2s0:wifi:unavailable\nlo:loopback:connected (externally)", "wlp2s0"},
		{"enp0s2:ethernet:connected", ""},
	} {
		if got := pickDevice(c.out); got != c.want {
			t.Errorf("pickDevice(%q) = %q, want %q", c.out, got, c.want)
		}
	}
}
