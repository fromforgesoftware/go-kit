package sqldb

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDSN(t *testing.T) {
	tests := []struct {
		name        string
		driver      DriverType
		opts        []ConnectionDSNOption
		envVars     map[string]string
		wantErr     bool
		expectedURL string
	}{
		{
			name:   "postgres with all fields",
			driver: DriverTypePostgres,
			opts: []ConnectionDSNOption{
				WithConnHost("localhost"),
				WithConnPort("5432"),
				WithConnUser("user"),
				WithConnPwd("pass"),
				WithConnDBName("mydb"),
				WithConnSSLMode("disable"),
			},
			expectedURL: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
		},
		{
			name:   "postgres with search path",
			driver: DriverTypePostgres,
			opts: []ConnectionDSNOption{
				WithConnHost("localhost"),
				WithConnPort("5432"),
				WithConnUser("user"),
				WithConnPwd("pass"),
				WithConnDBName("mydb"),
				WithConnSSLMode("disable"),
				WithConnSearchPath("myschema"),
			},
			expectedURL: "postgres://user:pass@localhost:5432/mydb?options=-c+search_path%3Dmyschema&sslmode=disable",
		},
		{
			name:    "invalid driver",
			driver:  "invalid",
			wantErr: true,
		},
		{
			name:   "missing fields",
			driver: DriverTypePostgres,
			opts: []ConnectionDSNOption{
				WithConnHost("localhost"),
			},
			wantErr: true,
		},
		{
			name:   "from env",
			driver: DriverTypePostgres,
			envVars: map[string]string{
				"DB_HOST":     "envhost",
				"DB_PORT":     "5432",
				"DB_USER":     "envuser",
				"DB_PASSWORD": "envpass",
				"DB_NAME":     "envdb",
				"DB_SSL":      "require",
			},
			// Default behavior reads from env if no opts provided (implied by defaultDSNOptions in NewDSN)
			// But note: NewDSN appends options. Explicit Env option will overwrite if passed.
			// Actually NewDSN adds defaultDSNOptions() which includes WithDSNConnFromEnv().
			// So provided opts override env. Here we provide NO opts, so it should read env.
			expectedURL: "postgres://envuser:envpass@envhost:5432/envdb?sslmode=require",
		},
		{
			name:   "env override by explicit opts",
			driver: DriverTypePostgres,
			envVars: map[string]string{
				"DB_HOST": "envhost",
			},
			opts: []ConnectionDSNOption{
				WithConnHost("overridden"),
				WithConnPort("5432"),
				WithConnUser("user"),
				WithConnPwd("pass"),
				WithConnDBName("db"),
				WithConnSSLMode("disable"),
			},
			expectedURL: "postgres://user:pass@overridden:5432/db?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Env
			os.Clearenv()
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// We need to ensure we clear env vars that might affect tests if not explicitly set in the test case
			// but os.Clearenv() is destructive to the whole process.
			// safe approach: t.Setenv for all keys used in NewDSN to empty if not in tt.envVars?
			// But NewDSN uses os.Getenv.
			// Better: t.Setenv sets it for the duration of the test.
			// To be safe, for the "no env" tests, we should probably explicitly un-set them or ensure they are empty.
			// Since we can't easily iterate all possible env vars, let's just assume pure unit test environment is clean OR explicit set empty.
			if len(tt.envVars) == 0 {
				t.Setenv("DB_HOST", "")
				t.Setenv("DB_PORT", "")
				t.Setenv("DB_USER", "")
				t.Setenv("DB_PASSWORD", "")
				t.Setenv("DB_NAME", "")
				t.Setenv("DB_SSL", "")
			}

			// If expecting success with no opts and no env, it will fail validation.
			// So "missing fields" test case above relies on empty env.

			u, err := NewDSN(tt.driver, tt.opts...)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, u.String())
		})
	}
}

func TestMustGenerateDSN(t *testing.T) {
	t.Run("panics on error", func(t *testing.T) {
		assert.Panics(t, func() {
			MustGenerateDSN("invalid_driver")
		})
	})

	t.Run("returns url on success", func(t *testing.T) {
		t.Setenv("DB_HOST", "localhost")
		t.Setenv("DB_PORT", "5432")
		t.Setenv("DB_USER", "user")
		t.Setenv("DB_PASSWORD", "pass")
		t.Setenv("DB_NAME", "db")
		t.Setenv("DB_SSL", "disable")

		assert.NotPanics(t, func() {
			u := MustGenerateDSN(DriverTypePostgres)
			assert.NotNil(t, u)
		})
	})
}
