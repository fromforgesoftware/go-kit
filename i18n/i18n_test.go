package i18n_test

import (
	"testing"
	"testing/fstest"

	"github.com/fromforgesoftware/go-kit/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFixture() *i18n.I18n {
	return i18n.NewI18n("en", "en", map[string]i18n.Translation{
		"en": {
			"common": map[string]any{
				"save":   "Save",
				"cancel": "Cancel",
				"mobile": "Mobile",
			},
			"shared": map[string]any{
				"enum": map[string]any{
					"role": map[string]any{
						"org.admin":    "Customer Administrator",
						"tenant.admin": "Tenant Administrator",
					},
				},
			},
			"welcome": map[string]any{
				"message": "Welcome, {{name}}!",
			},
		},
		"es": {
			"common": map[string]any{
				"save":   "Guardar",
				"mobile": "Móvil",
			},
			"welcome": map[string]any{
				"message": "¡Bienvenido, {{name}}!",
			},
		},
		"es_MX": {
			"common": map[string]any{
				"mobile": "Celular",
			},
		},
	})
}

func TestTranslateExactMatch(t *testing.T) {
	i := newFixture()
	assert.Equal(t, "Save", i.Translate("en", "common::save"))
	assert.Equal(t, "Guardar", i.Translate("es", "common::save"))
}

func TestTranslateRegionalFallback(t *testing.T) {
	i := newFixture()
	assert.Equal(t, "Celular", i.Translate("es_MX", "common::mobile"))
	assert.Equal(t, "Guardar", i.Translate("es_MX", "common::save"))
}

func TestTranslateFallsBackToFallbackLanguage(t *testing.T) {
	i := newFixture()
	assert.Equal(t, "Cancel", i.Translate("es", "common::cancel"))
}

func TestTranslateMissingKeyReturnsKey(t *testing.T) {
	i := newFixture()
	assert.Equal(t, "missing::key", i.Translate("en", "missing::key"))
}

func TestTranslateDotsInKeyNames(t *testing.T) {
	i := newFixture()
	assert.Equal(t,
		"Customer Administrator",
		i.Translate("en", "shared::enum::role::org.admin"),
	)
}

func TestTranslateInterpolates(t *testing.T) {
	i := newFixture()
	assert.Equal(t,
		"Welcome, John!",
		i.Translate("en", "welcome::message", map[string]string{"name": "John"}),
	)
}

func TestTranslateInterpolationLeavesUnknownPlaceholders(t *testing.T) {
	i := newFixture()
	assert.Equal(t,
		"Welcome, {{name}}!",
		i.Translate("en", "welcome::message", map[string]string{"other": "x"}),
	)
}

func TestHasTranslation(t *testing.T) {
	i := newFixture()
	assert.True(t, i.HasTranslation("en", "common::save"))
	assert.True(t, i.HasTranslation("es_MX", "common::save"))
	assert.False(t, i.HasTranslation("en", "no.such.key"))
}

func TestGetAvailableLanguages(t *testing.T) {
	i := newFixture()
	assert.Equal(t, []string{"en", "es", "es_MX"}, i.GetAvailableLanguages())
}

func TestGetKeysReturnsFullNamespaces(t *testing.T) {
	i := newFixture()
	keys := i.GetKeys("en", "common")
	assert.Equal(t, []string{"common::cancel", "common::mobile", "common::save"}, keys)
}

func TestGetKeysMergesAcrossFallbacks(t *testing.T) {
	i := newFixture()
	keys := i.GetKeys("es_MX", "common")
	// es_MX has mobile; es has save+mobile; en has save+cancel+mobile.
	assert.Equal(t, []string{"common::cancel", "common::mobile", "common::save"}, keys)
}

func TestGetKeyValuesReturnsLocalKeys(t *testing.T) {
	i := newFixture()
	got := i.GetKeyValues("en", "common")
	assert.Equal(t, map[string]string{
		"save":   "Save",
		"cancel": "Cancel",
		"mobile": "Mobile",
	}, got)
}

func TestGetKeyValuesPreservesDotsInKeys(t *testing.T) {
	i := newFixture()
	got := i.GetKeyValues("en", "shared::enum::role")
	assert.Equal(t, map[string]string{
		"org.admin":    "Customer Administrator",
		"tenant.admin": "Tenant Administrator",
	}, got)
}

func TestGetKeyValuesIgnoresNestedObjects(t *testing.T) {
	i := newFixture()
	got := i.GetKeyValues("en", "shared::enum")
	// "role" is a map, not a string — must be absent.
	assert.Empty(t, got)
}

func TestGetKeyValuesAppliesInterpolation(t *testing.T) {
	i := newFixture()
	got := i.GetKeyValues("en", "welcome", map[string]string{"name": "Ada"})
	assert.Equal(t, map[string]string{"message": "Welcome, Ada!"}, got)
}

func TestGetValuesSortedSlice(t *testing.T) {
	i := newFixture()
	got := i.GetValues("en", "shared::enum::role")
	require.Len(t, got, 2)
	assert.Equal(t, "org.admin", got[0].ID)
	assert.Equal(t, "Customer Administrator", got[0].Value)
	assert.Equal(t, "tenant.admin", got[1].ID)
}

func TestLoadFromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"locales/en.json": &fstest.MapFile{
			Data: []byte(`{"common":{"save":"Save"}}`),
		},
		"locales/es.json": &fstest.MapFile{
			Data: []byte(`{"common":{"save":"Guardar"}}`),
		},
	}
	inst, err := i18n.LoadFromFS(fsys, "locales", "en", "en")
	require.NoError(t, err)
	assert.Equal(t, "Save", inst.Translate("en", "common::save"))
	assert.Equal(t, "Guardar", inst.Translate("es", "common::save"))
}

func TestLoadFromFSRejectsBadJSON(t *testing.T) {
	fsys := fstest.MapFS{
		"locales/en.json": &fstest.MapFile{Data: []byte("not json")},
	}
	_, err := i18n.LoadFromFS(fsys, "locales", "en", "en")
	require.Error(t, err)
}
