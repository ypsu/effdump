// Package edbg (Effdump DeBuG) provides a helper function to aid debugging.
// Can be used to capture internal data into effdumptest and thus improve visibility into the internals.
package edbg

// Printf prints the printf formatted message to the debug log.
// Does nothing by default but tests can override it capture data.
var Printf = func(string, ...any) {}

// Reset resets Printf to do nothing.
func Reset() { Printf = func(string, ...any) {} }
