package edmain

import (
	"encoding/json"
	"fmt"
)

// Stringify converts any type to a string representation suitable to use in effects.
func Stringify(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	if s, ok := v.(error); ok {
		return s.Error()
	}
	switch v := v.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v)
	}

	js, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(js) + "\n"
}
