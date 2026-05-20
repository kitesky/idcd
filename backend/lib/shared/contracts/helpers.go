package contracts

import (
	"fmt"
	"strconv"
)

// formatBool encodes a bool as the canonical "true"/"false" string used
// by stream consumers. Stays a function (not a constant lookup) to keep
// the encoding rule in one place.
func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// parseInt64 accepts int / int64 / float64 / string forms.
// go-redis decodes XRange into map[string]interface{} where every value
// is a string, but tests construct maps directly with native int types.
// Both must work.
func parseInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		return int64(x), nil
	case string:
		if x == "" {
			return 0, nil
		}
		n, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("not an int64: %q", x)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

// parseBool accepts bool / "true"/"false" / "1"/"0".
// Tolerant of historical inconsistencies between producers.
func parseBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		switch x {
		case "true", "TRUE", "True", "1":
			return true, nil
		case "false", "FALSE", "False", "0", "":
			return false, nil
		default:
			return false, fmt.Errorf("not a bool: %q", x)
		}
	case int:
		return x != 0, nil
	case int64:
		return x != 0, nil
	case float64:
		return x != 0, nil
	default:
		return false, fmt.Errorf("unsupported type %T", v)
	}
}
