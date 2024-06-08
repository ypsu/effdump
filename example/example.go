// This is a barebones example for a fictional settings for a service.
// The service is deployed in multiple places such as Dublin, London, New York.
// Most deployments have the same settings, but some have different.
// effdump detects identical diffs and makes the reviews easier.
// Try changing the memory and observe the diffs.
package main

import (
	"fmt"
	"os"

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

func run() error {
	d := effdump.New()

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

	d.Add("template", template)

	d.Add("staging", mk(template, settings{
		"active": "true",
		"binary": "examplebinary/staging",
	}))

	// Production configs below.
	d.Add("dublin", mk(template, prod))
	d.Add("london", mk(template, prod))
	d.Add("paris", mk(template, prod))
	d.Add("newyork", mk(template, prod, settings{
		// Bump the limits in New York because most traffic happens there.
		"cpu_limit":    "8",
		"memory_limit": "64",
	}))

	d.Run("example-deployment-config")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
