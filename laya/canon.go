package laya

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
)

// Canonical returns a canonical encoding of a JSON document for STRUCTURAL
// equality (the comparison DESIGN D13 calls JCS): object keys sorted, no
// insignificant whitespace, and numbers compared by value — 2, 2.0 and 2e0
// encode the same; integers of any size compare exactly. Two documents are
// structurally equal iff their Canonical encodings are byte-equal.
func Canonical(doc []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("canonical: trailing data after the JSON document")
	}
	n, err := canonNumbers(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(n)
}

// CanonicalValue is Canonical for a Go value (marshalled first).
func CanonicalValue(v interface{}) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return Canonical(b)
}

func canonNumbers(v interface{}) (interface{}, error) {
	switch t := v.(type) {
	case json.Number:
		s, err := canonNumber(string(t))
		return json.Number(s), err
	case map[string]interface{}:
		for k, x := range t {
			c, err := canonNumbers(x)
			if err != nil {
				return nil, err
			}
			t[k] = c
		}
	case []interface{}:
		for i, x := range t {
			c, err := canonNumbers(x)
			if err != nil {
				return nil, err
			}
			t[i] = c
		}
	}
	return v, nil
}

// canonNumber: an integral value — whatever its spelling (12, 12.0, 1.2e1,
// 1e20) — becomes its exact decimal integer; any other value is compared
// as the nearest float64.
func canonNumber(lit string) (string, error) {
	f, _, err := big.ParseFloat(lit, 10, 4096, big.ToNearestEven)
	if err != nil {
		return "", fmt.Errorf("canonical: bad number %q: %v", lit, err)
	}
	if f.IsInt() {
		i, _ := f.Int(nil)
		return i.String(), nil
	}
	g, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		return "", fmt.Errorf("canonical: bad number %q: %v", lit, err)
	}
	return strconv.FormatFloat(g, 'g', -1, 64), nil
}
