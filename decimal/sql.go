package decimal

import "database/sql/driver"

// Value implements driver.Valuer — persists as a NUMERIC/string, exact.
func (d Decimal) Value() (driver.Value, error) { return d.d.Value() }

// Scan implements sql.Scanner for NUMERIC/string columns.
func (d *Decimal) Scan(v any) error { return d.d.Scan(v) }
