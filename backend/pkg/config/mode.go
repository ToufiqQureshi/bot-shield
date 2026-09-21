package config

import "fmt"

// Mode decides whether core.Guard acts on its scores or only records them.
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

// PolicyMode controls the decision strategy for incoming unscored traffic.
// PolicyBalanced passively allows clean traffic (score 0) without challenge,
// reserving challenges for elevated risk (25-99) and blocks for >=100.
// PolicyStrict requires a mandatory interstitial challenge for all initial traffic.
type PolicyMode int

const (
	PolicyBalanced PolicyMode = iota
	PolicyStrict
)

func (p PolicyMode) String() string {
	if p == PolicyStrict {
		return "strict"
	}
	return "balanced"
}

// ParsePolicy parses policy strings ("balanced", "strict").
func ParsePolicy(s string) (PolicyMode, error) {
	switch s {
	case "balanced", "":
		return PolicyBalanced, nil
	case "strict":
		return PolicyStrict, nil
	default:
		return PolicyBalanced, fmt.Errorf("unknown policy %q: want \"balanced\" or \"strict\"", s)
	}
}
