package sqldb

// DriverType defines the kind of database.
type DriverType string

const (
	// DriverTypePostgres defines postgres as the driver type being used to connect to the database.
	DriverTypePostgres DriverType = "postgres"
)

//nolint:gochecknoglobals // we need a way to control all the driver types so we can ensure in the valid function if a given drivertype does exist
var allDriverTypes = []DriverType{
	DriverTypePostgres,
}

func (t DriverType) valid() bool {
	for _, dt := range allDriverTypes {
		if t == dt {
			return true
		}
	}

	return false
}
