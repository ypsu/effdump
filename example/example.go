// This is a barebones example for a fictional settings for a service.
// The service is deployed in multiple places such as Dublin, London, New York.
// Most deployments have the same settings, but some have different.
// effdump detects identical diffs and makes the reviews easier.
// Try changing the memory and observe the diffs.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ypsu/effdump"
)

type settings map[string]string

// mk makes a new settings based on a series overrides.
func mk(overrides ...settings) settings {
	r := settings{}
	for _, o := range overrides {
		for k, v := range o {
			if v == "" {
				delete(r, k)
			} else {
				r[k] = v
			}
		}
	}
	return r
}

func atoi(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return v
}

func run() error {
	d := effdump.New()

	// add computes and saves the settings for a deployment based on a list of settings overrides.
	add := func(name string, overrides ...settings) {
		// Make variable substitutions.
		ss := mk(overrides...)
		for k, v := range ss {
			switch v {
			case "$NAME":
				ss[k] = name
			}
		}

		if ss["active"] == "false" {
			d.Add(name, ss)
			return
		}

		// Sanity check the active config.
		if got, want := atoi(ss["memory_limit"]), 32; got < want {
			ss["error"] += fmt.Sprintf("memory_limit = %d, want at least %d\n", got, want)
		}
		if ss["zone"] == "" {
			ss["error"] += fmt.Sprintf("missing zone\n")
		}

		d.Add(name, ss)
	}

	template := settings{
		"active":       "false",
		"name":         "$NAME",
		"binary":       "examplebinary/prod",
		"cpu_limit":    "4",
		"memory_limit": "32",
		"send_alerts":  "false",
	}
	prod := settings{
		"active":      "true",
		"send_alerts": "true",
	}
	euZone := settings{"zone": "eu"}
	usZone := settings{"zone": "us"}

	add("template", template)

	add("staging", mk(template, settings{
		"active": "true",
		"binary": "examplebinary/staging",
		"zone":   "test",
	}))

	// Production configs below.
	add("dublin", template, prod, euZone)
	add("london", template, prod, euZone)
	add("paris", template, prod, euZone)
	add("newyork", template, prod, usZone, settings{
		// Bump the limits in New York because most traffic happens there.
		"cpu_limit":    "8",
		"memory_limit": "64",
	})

	d.Run("example-deployment-config")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
