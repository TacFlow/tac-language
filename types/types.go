// Package types implements the Trust Type system for TAC Language.
//
// TAC does not have traditional data types (int, string).
// Instead, it has trust types that model the provenance and safety of data:
//
//	Secret      — Config store, user credentials; never echoed, never logged
//	Untrusted   — User input, web scrape; must be validated before use
//	Fact        — Memory store, verified sources; high confidence, persisted
//	Hallucinable — LLM output; may contain false information
//	Control     — Flow engine, internal state; read-only at agent level
//
// The type system enforces compile-time safety rules defined in SPEC §5.
package types

import "fmt"

// TrustType identifies the provenance and safety category of a value.
type TrustType string

const (
	Secret      TrustType = "Secret"
	Untrusted   TrustType = "Untrusted"
	Fact        TrustType = "Fact"
	Hallucinable TrustType = "Hallucinable"
	Control     TrustType = "Control"
)

// ValidTrustTypes returns all defined trust types.
func ValidTrustTypes() []TrustType {
	return []TrustType{Secret, Untrusted, Fact, Hallucinable, Control}
}

// IsValidTrustType checks if a string is a recognized trust type.
func IsValidTrustType(s string) bool {
	switch TrustType(s) {
	case Secret, Untrusted, Fact, Hallucinable, Control:
		return true
	}
	return false
}

// ConversionRule defines whether and how one trust type can convert to another.
type ConversionRule int

const (
	ConvertDirect     ConversionRule = iota // ✅ Direct conversion allowed
	ConvertExplicit                         // ⚠️ Requires explicit conversion function
	ConvertForbidden                        // ❌ Conversion is forbidden
)

// conversionTable maps source→target trust type conversions.
// Based on SPEC v0.3 §13.2 Required conversion rules.
//
//	Untrusted → Fact         = explicit (validate)
//	Hallucinable → Fact      = explicit (verify)
//	Secret → Hallucinable    = forbidden
//	Secret → Untrusted       = forbidden
//	Secret → Fact            = explicit (policy-controlled)
//	Control → Fact           = explicit (authorize)
//	Fact → Untrusted         = direct (allowed but discouraged)
//	Fact → Hallucinable      = direct
var conversionTable = map[TrustType]map[TrustType]ConversionRule{
	Secret: {
		Secret:       ConvertDirect,
		Untrusted:    ConvertForbidden,
		Fact:         ConvertExplicit, // policy-controlled only
		Hallucinable: ConvertForbidden,
		Control:      ConvertForbidden,
	},
	Untrusted: {
		Secret:       ConvertForbidden,
		Untrusted:    ConvertDirect,
		Fact:         ConvertExplicit, // require validate()
		Hallucinable: ConvertExplicit, // require sanitize()
		Control:      ConvertForbidden,
	},
	Fact: {
		Secret:       ConvertForbidden,
		Untrusted:    ConvertDirect,   // allowed but discouraged
		Fact:         ConvertDirect,
		Hallucinable: ConvertDirect,
		Control:      ConvertForbidden,
	},
	Hallucinable: {
		Secret:       ConvertForbidden,
		Untrusted:    ConvertDirect,
		Fact:         ConvertExplicit, // require verify()
		Hallucinable: ConvertDirect,
		Control:      ConvertForbidden,
	},
	Control: {
		Secret:       ConvertForbidden,
		Untrusted:    ConvertForbidden,
		Fact:         ConvertExplicit, // require explicit authorization
		Hallucinable: ConvertForbidden,
		Control:      ConvertDirect,
	},
}

// CanConvert checks if source trust type can be converted to target.
// Returns the conversion rule and whether it's allowed.
func CanConvert(from, to TrustType) (ConversionRule, bool) {
	rules, ok := conversionTable[from]
	if !ok {
		return ConvertForbidden, false
	}
	rule, ok := rules[to]
	if !ok {
		return ConvertForbidden, false
	}
	return rule, rule != ConvertForbidden
}

// RequiresConversion returns true if the conversion requires an explicit
// conversion function (validate, verify, sanitize).
func RequiresConversion(from, to TrustType) bool {
	rule, ok := CanConvert(from, to)
	return ok && rule == ConvertExplicit
}

// ConversionFunction returns the name of the required conversion function
// for a specific conversion, if one is needed.
func ConversionFunction(from, to TrustType) string {
	switch {
	case from == Untrusted && to == Fact:
		return "validate"
	case from == Untrusted && to == Hallucinable:
		return "sanitize"
	case from == Hallucinable && to == Fact:
		return "verify"
	case from == Secret && to == Fact:
		return "authorize"
	case from == Control && to == Fact:
		return "authorize"
	default:
		return ""
	}
}

// InferTrustType infers a trust type from the origin of a value.
func InferTrustType(origin string) TrustType {
	switch origin {
	case "user_input", "web_search", "web_scrape", "user_message":
		return Untrusted
	case "memory_search", "memory_store", "config_get":
		return Fact
	case "llm.chat", "llm.classify", "vision.analyze", "vision.generate":
		return Hallucinable
	case "config_get.secret", "api_key":
		return Secret
	case "flow.run", "flow.status", "swarm_status", "agent_task":
		return Control
	default:
		return Untrusted // Conservative default
	}
}

// ValidateConversion checks that a type conversion is allowed and returns
// a descriptive error if it isn't.
func ValidateConversion(from, to TrustType, context string) error {
	rule, allowed := CanConvert(from, to)
	if !allowed {
		return fmt.Errorf("%s: cannot convert %s → %s (forbidden)",
			context, from, to)
	}
	if rule == ConvertExplicit {
		fn := ConversionFunction(from, to)
		return fmt.Errorf("%s: %s → %s requires explicit %s()",
			context, from, to, fn)
	}
	return nil
}

// String returns a human-readable description of the trust type.
func (tt TrustType) String() string {
	switch tt {
	case Secret:
		return "Secret (credentials — never echoed or logged)"
	case Untrusted:
		return "Untrusted (user/web input — must validate before critical use)"
	case Fact:
		return "Fact (verified knowledge — high confidence)"
	case Hallucinable:
		return "Hallucinable (LLM output — may contain falsehoods)"
	case Control:
		return "Control (flow engine state — read-only)"
	default:
		return string(tt)
	}
}
