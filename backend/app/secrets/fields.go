package secrets

import (
	"encoding/json"
	"fmt"
)

// Sentinel is what a masked credential reads as on the wire. A form that
// sends it back is asking to keep the stored value.
const Sentinel = "********"

// EncryptFields encrypts the named string fields of a JSON object. Values
// that are empty or already encrypted are left alone, so running it over a
// stored row is a no-op.
func EncryptFields(cfg json.RawMessage, fields []string) (json.RawMessage, error) {
	return transformFields(cfg, fields, func(_ string, value string) (string, error) {
		if value == "" || IsEncrypted(value) {
			return value, nil
		}
		return Encrypt([]byte(value))
	})
}

func DecryptFields(cfg json.RawMessage, fields []string) (json.RawMessage, error) {
	return transformFields(cfg, fields, func(_ string, value string) (string, error) {
		if !IsEncrypted(value) {
			return value, nil
		}
		plaintext, err := Decrypt(value)
		return string(plaintext), err
	})
}

// MaskFields replaces every set credential with Sentinel and returns the
// names of the fields it masked.
func MaskFields(cfg json.RawMessage, fields []string) (json.RawMessage, []string, error) {
	masked := []string{}
	out, err := transformFields(cfg, fields, func(field string, value string) (string, error) {
		if value == "" {
			return value, nil
		}
		masked = append(masked, field)
		return Sentinel, nil
	})
	return out, masked, err
}

// KeepStoredFields substitutes the stored value for every incoming credential
// that carries Sentinel, which is how an edit keeps a credential the form
// never displayed. A sentinel with nothing stored behind it becomes empty so
// the owner's validation reports the missing credential.
func KeepStoredFields(incoming, stored json.RawMessage, fields []string) (json.RawMessage, error) {
	var storedValues map[string]json.RawMessage
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &storedValues); err != nil {
			return nil, err
		}
	}
	return transformFields(incoming, fields, func(field string, value string) (string, error) {
		if value != Sentinel {
			return value, nil
		}
		var storedValue string
		if raw, ok := storedValues[field]; ok {
			json.Unmarshal(raw, &storedValue)
		}
		return storedValue, nil
	})
}

func transformFields(cfg json.RawMessage, fields []string, transform func(field, value string) (string, error)) (json.RawMessage, error) {
	if len(fields) == 0 || len(cfg) == 0 {
		return cfg, nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(cfg, &values); err != nil {
		return nil, err
	}
	changed := false
	for _, field := range fields {
		raw, ok := values[field]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		next, err := transform(field, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field, err)
		}
		if next == value {
			continue
		}
		encoded, err := json.Marshal(next)
		if err != nil {
			return nil, err
		}
		values[field] = encoded
		changed = true
	}
	if !changed {
		return cfg, nil
	}
	return json.Marshal(values)
}
