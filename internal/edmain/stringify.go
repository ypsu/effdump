package edmain

import "fmt"

// Stringify converts any type to a string representation suitable to use in effects.
func Stringify(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	if bs, ok := v.([]byte); ok {
		return string(bs)
	}
	return fmt.Sprint(v)
}
