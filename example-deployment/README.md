# Deployment example for effdump

effdump shines when you manage many very similar generated configs.
One common example is a web service's deployment configs.
Imagine a web service that is replicated in many cities.
Each replica has similar resource requirements and command line flags.
But some replicas might have overrides for some of these attributes.

This example sketches a simple framework for managing such configs in Go and then it shows how to use effdump to make reviews easier.

## Configuration management

Define the attributes our server can have:

```
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
```

Each replica will be defined by such a struct.
But to make things easy to describe in a succint manner, create a helper function for each attribute:

```
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
...
```

These functions return a function that changes a field (except for reset which overwrites all fields).
The advantage of this form is that all these functions have the same type signature.
They are easy to combine.
Here's another helper function that makes use of this feature:

```
func seq(seq ...func(*config)) func(*config) {
	return func(c *config) {
		for _, fn := range seq {
			fn(c)
		}
	}
}
```

This allows creating combined overrides that set multiple fields with a single function call such as this one:

```
prodDeployment = seq(active(true), alerting(true))

// Appying the "prodDeployment" attributes on a config is just a function call:
prodDeployment(newYorkConfig)
```

## Fill the effdump

Create an effdump to make it easier to see the effects of the deployment config changes:

```
	d := effdump.New("exampledeployment")
```

The various "effects" have to be added individually to the dump.
Create a convenience function to add a config along with a set of overrides.

```
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

		d.Add(name, *cfg)
	}
```

It's possible to add various other features and safety checks into this function.
They are omitted for brevity.
See the full function in the source.

Create a template config and add it to the effdump:

```
	template := reset(config{
		Active:     false,
		Name:       "$NAME",
		BinaryFile: "examplebinary/prod",
		CPURate:    4,
		MemGB:      32,
		Alerting:   false,
	})
	add("template", template)
```

Now add the staging environment's config:

```
	add("staging", template, active(true), binaryFile("examplebinary/staging"), zone("test"))
```

Define helper overrides for prod environments and define the production environments using them:

```
	prod := seq(active(true), alerting(true))
	euZone := zone("eu")
	usZone := zone("us")

	// Production configs below.
	add("dublin", template, prod, euZone)
	add("london", template, prod, euZone)
	add("paris", template, prod, euZone)
	add("newyork", template, prod, usZone,
		// Bump the limits in New York because most traffic happens there.
		cpuRate(8), memGB(64),
	)
```

Now the effdump is filled.
Pass the execution to the effdump library to do the rest:

```
	d.Run(context.Background())
```

## Examine the effdump

TODO