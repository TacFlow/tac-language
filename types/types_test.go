package types

import (
	"testing"
)

func TestCanConvert(t *testing.T) {
	tests := []struct {
		from, to TrustType
		rule     ConversionRule
		allowed  bool
	}{
		{Secret, Secret, ConvertDirect, true},
		{Secret, Untrusted, ConvertForbidden, false},
		{Untrusted, Fact, ConvertExplicit, true},
		{Untrusted, Hallucinable, ConvertExplicit, true},
		{Fact, Hallucinable, ConvertDirect, true},
		{Hallucinable, Fact, ConvertExplicit, true},
		{Hallucinable, Untrusted, ConvertDirect, true},
		{Control, Fact, ConvertForbidden, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			rule, allowed := CanConvert(tt.from, tt.to)
			if allowed != tt.allowed {
				t.Errorf("CanConvert(%s, %s): expected allowed=%v, got %v", tt.from, tt.to, tt.allowed, allowed)
			}
			if rule != tt.rule {
				t.Errorf("CanConvert(%s, %s): expected rule=%d, got %d", tt.from, tt.to, tt.rule, rule)
			}
		})
	}
}

func TestInferTrustType(t *testing.T) {
	tests := []struct {
		origin   string
		expected TrustType
	}{
		{"user_input", Untrusted},
		{"web_search", Untrusted},
		{"memory_search", Fact},
		{"config_get", Fact},
		{"llm.chat", Hallucinable},
		{"flow.run", Control},
		{"unknown_source", Untrusted}, // conservative default
	}

	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			result := InferTrustType(tt.origin)
			if result != tt.expected {
				t.Errorf("InferTrustType(%q): expected %s, got %s", tt.origin, tt.expected, result)
			}
		})
	}
}

func TestConversionFunction(t *testing.T) {
	tests := []struct {
		from, to TrustType
		fn       string
	}{
		{Untrusted, Fact, "validate"},
		{Untrusted, Hallucinable, "sanitize"},
		{Hallucinable, Fact, "verify"},
		{Fact, Hallucinable, ""},
		{Secret, Secret, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			result := ConversionFunction(tt.from, tt.to)
			if result != tt.fn {
				t.Errorf("ConversionFunction(%s, %s): expected %q, got %q", tt.from, tt.to, tt.fn, result)
			}
		})
	}
}

func TestValidTrustTypes(t *testing.T) {
	types := ValidTrustTypes()
	if len(types) != 5 {
		t.Errorf("expected 5 trust types, got %d", len(types))
	}
	seen := make(map[TrustType]bool)
	for _, tt := range types {
		if seen[tt] {
			t.Errorf("duplicate trust type: %s", tt)
		}
		seen[tt] = true
	}
}
