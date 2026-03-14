package output

import "github.com/alpkeskin/gotoon"

// ToTOON converts a JSON-compatible value to TOON format (~40% token savings
// over JSON with no information loss). See https://toonformat.dev
func ToTOON(v any) ([]byte, error) {
	s, err := gotoon.Encode(v, gotoon.WithIndent(2))
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}
