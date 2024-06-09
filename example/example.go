// Binary example generates the deployment config effects for a hypothetical service.
//
// This is a barebones example for a fictional settings for a service.
// The service is deployed in multiple places such as Dublin, London, New York.
// Most deployments have the same settings, but some have different.
// effdump detects identical diffs and makes the reviews easier.
// Try changing the memory and observe the diffs.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ypsu/effdump"
)

type config struct {
	Active     bool
	Name       string
	Zone       string
	BinaryFile string
	CPURate    float64
	MemGB      float64
	Alerting   bool

	// Error, if non-empty, the config is invalid.
	Error string
}

func reset(v config) func(*config) {
	return func(c *config) { *c = v }
}
func active(v bool) func(*config) {
	return func(c *config) { c.Active = v }
}
func name(v string) func(*config) {
	return func(c *config) { c.Name = v }
}
func zone(v string) func(*config) {
	return func(c *config) { c.Zone = v }
}
func binaryFile(v string) func(*config) {
	return func(c *config) { c.BinaryFile = v }
}
func cpuRate(v float64) func(*config) {
	return func(c *config) { c.CPURate = v }
}
func memGB(v float64) func(*config) {
	return func(c *config) { c.MemGB = v }
}
func alerting(v bool) func(*config) {
	return func(c *config) { c.Alerting = v }
}
func seq(seq ...func(*config)) func(*config) {
	return func(c *config) {
		for _, fn := range seq {
			fn(c)
		}
	}
}

func run() error {
	d := effdump.New("example-deployment-config")

	// add computes and saves deployment config based on a list of config overrides.
	add := func(name string, overrides ...func(*config)) {
		// Apply the overrides.
		cfg := &config{}
		for _, o := range overrides {
			o(cfg)
		}

		// Resolve late stage references in select fields.
		if cfg.Name == "$NAME" {
			cfg.Name = name
		}

		if !cfg.Active {
			d.Add(name, cfg)
			return
		}

		// Sanity check the active configs.
		if got, want := cfg.MemGB, 32.0; got < want {
			cfg.Error += fmt.Sprintf("MemGB = %0.3f, want at least %.0f\n", got, want)
		}
		if cfg.Zone == "" {
			cfg.Error += fmt.Sprintf("missing zone\n")
		}

		d.Add(name, *cfg)
	}

	template := reset(config{
		Active:     false,
		Name:       "$NAME",
		BinaryFile: "examplebinary/prod",
		CPURate:    4,
		MemGB:      32,
		Alerting:   false,
	})
	prod := seq(active(true), alerting(true))
	euZone := zone("eu")
	usZone := zone("us")

	add("template", template)
	add("staging", template, active(true), binaryFile("examplebinary/staging"), zone("test"))

	// Production configs below.
	add("dublin", template, prod, euZone)
	add("london", template, prod, euZone)
	add("paris", template, prod, euZone)
	add("newyork", template, prod, usZone,
		// Bump the limits in New York because most traffic happens there.
		cpuRate(8), memGB(64),
	)

	d.Run(context.Background())
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
