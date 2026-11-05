package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jacobzipper/vibian/internal/doctor"
)

var marks = map[doctor.Status]string{
	doctor.OK:   "\033[32m✓\033[0m",
	doctor.Warn: "\033[33m!\033[0m",
	doctor.Fail: "\033[31m✗\033[0m",
	doctor.Info: "\033[2m·\033[0m",
}

func cmdDoctor(args []string) error {
	if len(args) > 0 && args[0] == "--json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(doctor.Run())
	}
	bad := 0
	for _, g := range doctor.Groups {
		fmt.Printf("\n\033[1m%s\033[0m\n", g.Name)
		for _, check := range g.Checks {
			c := check()
			fmt.Printf("  %s %-16s %s\n", marks[c.Status], c.Name, c.Detail)
			if c.Fix != "" {
				fmt.Printf("    \033[2mfix: %s\033[0m\n", c.Fix)
			}
			if c.Status == doctor.Fail {
				bad++
			}
		}
	}
	fmt.Println()
	if bad > 0 {
		return fmt.Errorf("%d check(s) failed", bad)
	}
	return nil
}
