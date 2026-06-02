package rest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dummyMemberInput is the DTO read from the bulk-create body.
type dummyMemberInput struct {
	*dummyMember
}

func TestNewJsonApiBulkCreateHandler(t *testing.T) {
	created := []*dummyMember{}
	cmdFn := stubBatchCreator(t, func(in []*dummyMember) []*dummyMember {
		created = in
		// Pretend the DB assigned ids; mutate in place so the response shows them.
		for i, m := range in {
			m.RID = "M" + string(rune('0'+i))
		}
		return in
	})

	decoder := func(d *dummyMember) *dummyMember { return d }
	encoder := func(m *dummyMember) *dummyMember { return m }

	h := rest.NewJsonApiBulkCreateHandler(cmdFn, decoder, encoder)

	body := `{"data":[
		{"type":"workspaceMembers","attributes":{"accountId":"acc-1","role":"admin"}},
		{"type":"workspaceMembers","attributes":{"accountId":"acc-2","role":"viewer"}}
	]}`

	req := httptest.NewRequest(http.MethodPost, "/v1/members", strings.NewReader(body))
	req.Header.Set("Content-Type", jsonapi.MediaType)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, created, 2)
	assert.Contains(t, rec.Body.String(), `"id":"M0"`)
	assert.Contains(t, rec.Body.String(), `"id":"M1"`)
}

// stubBatchCreator wraps the test's transform func into the
// usecase.CreatorBatch shape kit expects. Returns a thin adapter
// that satisfies the generic constraint.
func stubBatchCreator(_ *testing.T, fn func([]*dummyMember) []*dummyMember) *batchCreator {
	return &batchCreator{fn: fn}
}

type batchCreator struct {
	fn func([]*dummyMember) []*dummyMember
}

func (b *batchCreator) CreateBatch(_ context.Context, in []*dummyMember) ([]*dummyMember, error) {
	return b.fn(in), nil
}

// --- BulkUpdate -------------------------------------------------------------

type stubFilter struct {
	Status string `json:"status"`
}
type stubPatch struct {
	NewStatus string `json:"newStatus"`
}
type stubBulkCmd struct {
	Filter stubFilter
	Patch  stubPatch
}

func TestNewJsonApiBulkUpdateHandler(t *testing.T) {
	var seen stubBulkCmd
	cmdFn := func(_ context.Context, cmd stubBulkCmd) (int, error) {
		seen = cmd
		return 42, nil
	}
	cmdFromReq := func(f stubFilter, p stubPatch) stubBulkCmd {
		return stubBulkCmd{Filter: f, Patch: p}
	}

	h := rest.NewJsonApiBulkUpdateHandler(cmdFn, cmdFromReq)

	body := `{"filter":{"status":"overdue"},"patch":{"newStatus":"paid"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/invoices:batchUpdate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "overdue", seen.Filter.Status)
	assert.Equal(t, "paid", seen.Patch.NewStatus)
	assert.Contains(t, rec.Body.String(), `"affectedCount":42`)
}

// --- BulkDelete -------------------------------------------------------------

type stubBulkDeleteCmd struct {
	Filter stubFilter
}

func TestNewJsonApiBulkDeleteHandler(t *testing.T) {
	var seen stubBulkDeleteCmd
	cmdFn := func(_ context.Context, cmd stubBulkDeleteCmd) (int, error) {
		seen = cmd
		return 7, nil
	}
	cmdFromReq := func(f stubFilter) stubBulkDeleteCmd {
		return stubBulkDeleteCmd{Filter: f}
	}

	h := rest.NewJsonApiBulkDeleteHandler(cmdFn, cmdFromReq)

	body := `{"filter":{"status":"draft"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/invoices:batchDelete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "draft", seen.Filter.Status)
	assert.Contains(t, rec.Body.String(), `"affectedCount":7`)
}

// --- AtomicOperations -------------------------------------------------------

func TestNewJsonApiAtomicOperationsHandler(t *testing.T) {
	dispatcher := func(_ context.Context, ops []rest.AtomicOperation) ([]rest.AtomicOperationResult, error) {
		require.Len(t, ops, 2)
		assert.Equal(t, "add", ops[0].Op)
		assert.Equal(t, "remove", ops[1].Op)
		require.NotNil(t, ops[1].Ref)
		assert.Equal(t, "X1", ops[1].Ref.ID)
		return []rest.AtomicOperationResult{
			{Data: map[string]any{"type": "things", "id": "NEW1"}},
			{Data: nil}, // remove has no data
		}, nil
	}

	h := rest.NewJsonApiAtomicOperationsHandler(dispatcher)

	body := `{
		"atomic:operations": [
			{"op":"add","data":{"type":"things","attributes":{"name":"a"}}},
			{"op":"remove","ref":{"type":"things","id":"X1"}}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/operations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got rest.AtomicOperationsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Results, 2)
	assert.Equal(t, "NEW1", got.Results[0].Data["id"])
}

func TestNewJsonApiAtomicOperationsHandlerDispatcherError(t *testing.T) {
	dispatcher := func(_ context.Context, _ []rest.AtomicOperation) ([]rest.AtomicOperationResult, error) {
		return nil, http.ErrAbortHandler // any non-kit error → 500
	}
	h := rest.NewJsonApiAtomicOperationsHandler(dispatcher)
	req := httptest.NewRequest(http.MethodPost, "/x",
		strings.NewReader(`{"atomic:operations":[{"op":"add","data":{}}]}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
