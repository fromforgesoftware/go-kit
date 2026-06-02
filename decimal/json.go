package decimal

import (
	"strings"

	"github.com/alpacahq/alpacadecimal"
)

// MarshalJSON always emits a quoted string (never a float) so a JSON consumer
// can't silently lose precision parsing the value as a number.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.d.String() + `"`), nil
}

// UnmarshalJSON accepts a quoted string or a bare number; "" / null decode to zero.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		d.d = alpacadecimal.Decimal{}
		return nil
	}
	dec, err := alpacadecimal.NewFromString(s)
	if err != nil {
		return err
	}
	d.d = dec
	return nil
}
