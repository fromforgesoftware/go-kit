package inheritance_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/config/inheritance"
)

type cfg struct {
	CellSize      inheritance.Optional[float32]           `cfg:"cell_size,required"`
	WalkableSlope inheritance.Optional[float32]           `cfg:"walkable_slope"`
	Tags          inheritance.Optional[[]string]          `cfg:"tags,merge=append"`
	Labels        inheritance.Optional[map[string]string] `cfg:"labels,merge=union"`
}

func TestOverrideTakesHighestPriority(t *testing.T) {
	r := inheritance.NewResolver[cfg]()
	r.AddLayer(inheritance.Layer[cfg]{
		Name: "defaults",
		Data: cfg{CellSize: inheritance.Set[float32](0.3), WalkableSlope: inheritance.Set[float32](40)},
	})
	r.AddLayer(inheritance.Layer[cfg]{
		Name: "tile",
		Data: cfg{CellSize: inheritance.Set[float32](0.1)},
	})
	out, errs := r.Resolve()
	require.Empty(t, errs)
	cs, _ := out.CellSize.Get()
	assert.Equal(t, float32(0.1), cs)
	ws, _ := out.WalkableSlope.Get()
	assert.Equal(t, float32(40), ws)
}

func TestRequiredFieldErrorsWhenUnset(t *testing.T) {
	r := inheritance.NewResolver[cfg]()
	r.AddLayer(inheritance.Layer[cfg]{Data: cfg{WalkableSlope: inheritance.Set[float32](30)}})
	_, errs := r.Resolve()
	require.Len(t, errs, 1)
	assert.Equal(t, "cell_size", errs[0].Field)
	assert.Contains(t, errs[0].Reason, "required")
}

func TestMergeAppendConcatenates(t *testing.T) {
	r := inheritance.NewResolver[cfg]()
	r.AddLayer(inheritance.Layer[cfg]{
		Data: cfg{CellSize: inheritance.Set[float32](0.3), Tags: inheritance.Set([]string{"low", "static"})},
	})
	r.AddLayer(inheritance.Layer[cfg]{
		Data: cfg{Tags: inheritance.Set([]string{"high"})},
	})
	out, errs := r.Resolve()
	require.Empty(t, errs)
	tags, _ := out.Tags.Get()
	// Lower-priority entries appear first, higher-priority second.
	assert.Equal(t, []string{"low", "static", "high"}, tags)
}

func TestMergeUnionMergesMaps(t *testing.T) {
	r := inheritance.NewResolver[cfg]()
	r.AddLayer(inheritance.Layer[cfg]{
		Data: cfg{
			CellSize: inheritance.Set[float32](0.3),
			Labels:   inheritance.Set(map[string]string{"region": "eu", "tier": "low"}),
		},
	})
	r.AddLayer(inheritance.Layer[cfg]{
		Data: cfg{
			Labels: inheritance.Set(map[string]string{"tier": "high", "extra": "yes"}),
		},
	})
	out, errs := r.Resolve()
	require.Empty(t, errs)
	labels, _ := out.Labels.Get()
	assert.Equal(t, map[string]string{"region": "eu", "tier": "high", "extra": "yes"}, labels)
}

func TestOptionalJSONRoundTrip(t *testing.T) {
	type wrap struct {
		Name inheritance.Optional[string] `json:"name"`
		Hits inheritance.Optional[int]    `json:"hits"`
	}
	src := wrap{Name: inheritance.Set("alice")} // Hits unset
	data, err := json.Marshal(src)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"alice","hits":null}`, string(data))

	var got wrap
	require.NoError(t, json.Unmarshal(data, &got))
	n, ok := got.Name.Get()
	assert.True(t, ok)
	assert.Equal(t, "alice", n)
	_, ok = got.Hits.Get()
	assert.False(t, ok)
}

func TestOptionalOrDefault(t *testing.T) {
	assert.Equal(t, "fallback", inheritance.Unset[string]().OrDefault("fallback"))
	assert.Equal(t, "set", inheritance.Set("set").OrDefault("fallback"))
}

func TestRootMustBeStruct(t *testing.T) {
	r := inheritance.NewResolver[int]()
	r.AddLayer(inheritance.Layer[int]{Data: 1})
	_, errs := r.Resolve()
	require.Len(t, errs, 1)
	assert.Equal(t, "_root", errs[0].Field)
}
