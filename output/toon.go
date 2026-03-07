package output

import "github.com/alpkeskin/gotoon"

// ToTOON converts a JSON-compatible value to TOON format.
//
// TOON (Token-Oriented Object Notation) provides ~40% token savings compared
// to JSON with no information loss. Apps always output JSON; the OS handles
// conversion via this function.
//
// See https://toonformat.dev for the specification.
func ToTOON(v any) ([]byte, error) {
	s, err := gotoon.Encode(v, gotoon.WithIndent(2))
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}
