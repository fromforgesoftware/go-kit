package sqldb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDriverTypeValid(t *testing.T) {
	tests := []struct {
		name   string
		driver DriverType
		want   bool
	}{
		{
			name:   "postgres is valid",
			driver: DriverTypePostgres,
			want:   true,
		},
		{
			name:   "empty is invalid",
			driver: "",
			want:   false,
		},
		{
			name:   "unknown is invalid",
			driver: "mysql",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.driver.valid())
		})
	}
}
