package main

import (
	"reflect"
	"testing"
)

func TestParseOrphans(t *testing.T) {
	out := `greetd.service not-found active running greetd.service
session-3.scope not-found active running Session 3 of User rel
ssh.service loaded active running OpenBSD Secure Shell server
gone.service not-found inactive dead gone.service`
	if got, want := parseOrphans(out), []string{"greetd.service"}; !reflect.DeepEqual(got, want) {
		t.Errorf("parseOrphans = %q, want %q", got, want)
	}
}
