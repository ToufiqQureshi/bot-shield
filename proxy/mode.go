package proxy

import "fmt"

// Mode decides whether Guard acts on its scores or only records them.
// Shadow mode lets a client see what bot-shield would have done to
// their real traffic without risking a single blocked customer.
type Mode int

const (
	ModeEnforce Mode = iota
	ModeShadow
)

func (m Mode) String() string {
	if m == ModeShadow {
		return "shadow"
	}
	return "enforce"
}

// ParseMode reads the operator's chosen mode. An unrecognised value is
// an error rather than a default, because silently running in the
// wrong mode is the exact failure this feature must not have.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "enforce":
		return ModeEnforce, nil
	case "shadow":
		return ModeShadow, nil
	default:
		return ModeEnforce, fmt.Errorf("unknown mode %q: want \"enforce\" or \"shadow\"", s)
	}
}
