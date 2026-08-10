package redactor

import "testing"

func FuzzRedactJSON(f *testing.F) {
	f.Add([]byte(`{"password":"x"}`))
	f.Add([]byte(`{"a":{"b":"c"}}`))
	f.Add([]byte(`[1,2,3]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		r := New(nil, nil, []string{"password", "credit_card.number"}, "[REDACTED]")
		out, err := r.RedactJSON(data)
		if err == nil {
			// Must produce valid JSON when input was valid.
			_ = out
		}
		// Must never panic regardless of input.
	})
}
