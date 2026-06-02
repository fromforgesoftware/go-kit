package rest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invokeCommand wires a Command handler against a mux so path-param
// routing matches the production wiring, then issues a request.
func invokeCommand(t *testing.T, h http.Handler, method, urlPattern, requestURL, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(method+" "+urlPattern, h)

	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, requestURL, readerOrNil(rdr))
	if body != "" {
		req.Header.Set("Content-Type", jsonapi.MediaType)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func readerOrNil(r *strings.Reader) *strings.Reader {
	if r == nil {
		return strings.NewReader("")
	}
	return r
}

// dummyInviteRequest is the wire DTO used by the test decoder.
// jsonapi-tagged so jsonapi.UnmarshalPayload knows how to parse it.
type dummyInviteRequest struct {
	resource.RestDTO
	RAccountID string `jsonapi:"attr,accountId"`
	RRole      string `jsonapi:"attr,role"`
}

func (d *dummyInviteRequest) AccountID() string { return d.RAccountID }
func (d *dummyInviteRequest) Role() string      { return d.RRole }

// dummyInviteCommand is what the usecase consumes.
type dummyInviteCommand struct {
	WorkspaceID string
	AccountID   string
	Role        string
}

// dummyMember is the resource returned by the usecase. Implements
// resource.Resource so it can be a Command handler R + DTO target.
type dummyMember struct {
	resource.RestDTO
	RWorkspaceID string `jsonapi:"attr,workspaceId"`
	RAccountID   string `jsonapi:"attr,accountId"`
	RRole        string `jsonapi:"attr,role"`
}

func newDummyMember(id, ws, acc, role string) *dummyMember {
	m := &dummyMember{RWorkspaceID: ws, RAccountID: acc, RRole: role}
	m.RID = id
	m.RType = "workspaceMembers"
	m.RTimestamps = &resource.TimestampDTO{RCreatedAt: time.Now(), RUpdatedAt: time.Now()}
	return m
}

func TestNewJsonApiCommandHandlerHappyPath(t *testing.T) {
	cmdFn := func(_ context.Context, cmd dummyInviteCommand) (*dummyMember, error) {
		// Verify the decoder mapped path + body into the Command.
		assert.Equal(t, "W1", cmd.WorkspaceID)
		assert.Equal(t, "acc-9", cmd.AccountID)
		assert.Equal(t, "admin", cmd.Role)
		return newDummyMember("M1", cmd.WorkspaceID, cmd.AccountID, cmd.Role), nil
	}

	decoder := func(req *http.Request) (dummyInviteCommand, error) {
		body, err := rest.UnmarshalPayloadFromRequest[*dummyInviteRequest](req)
		if err != nil {
			return dummyInviteCommand{}, err
		}
		return dummyInviteCommand{
			WorkspaceID: req.PathValue("id"),
			AccountID:   body.AccountID(),
			Role:        body.Role(),
		}, nil
	}

	encoder := func(m *dummyMember) *dummyMember { return m } // identity — already a DTO

	h := rest.NewJsonApiCommandHandler(cmdFn, decoder, encoder)

	body := `{"data":{"type":"workspaceMembers","attributes":{"accountId":"acc-9","role":"admin"}}}`
	rec := invokeCommand(t, h, http.MethodPost, "/v1/workspaces/{id}/members", "/v1/workspaces/W1/members", body)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Contains(t, rec.Body.String(), `"workspaceId":"W1"`)
	require.Contains(t, rec.Body.String(), `"accountId":"acc-9"`)
}

func TestNewJsonApiCommandHandlerDecoderError(t *testing.T) {
	cmdFn := func(_ context.Context, _ dummyInviteCommand) (*dummyMember, error) {
		t.Fatal("usecase should not be invoked when the decoder errors")
		return nil, nil
	}
	decoder := func(_ *http.Request) (dummyInviteCommand, error) {
		return dummyInviteCommand{}, errors.InvalidArgument("bad body")
	}
	encoder := func(m *dummyMember) *dummyMember { return m }

	h := rest.NewJsonApiCommandHandler(cmdFn, decoder, encoder)
	rec := invokeCommand(t, h, http.MethodPost, "/v1/workspaces/{id}/members", "/v1/workspaces/W1/members", `{}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNewJsonApiCommandHandlerUsecaseError(t *testing.T) {
	cmdFn := func(_ context.Context, _ dummyInviteCommand) (*dummyMember, error) {
		return nil, errors.NotFound("workspace", "W1")
	}
	decoder := func(req *http.Request) (dummyInviteCommand, error) {
		return dummyInviteCommand{WorkspaceID: req.PathValue("id")}, nil
	}
	encoder := func(m *dummyMember) *dummyMember { return m }

	h := rest.NewJsonApiCommandHandler(cmdFn, decoder, encoder)
	rec := invokeCommand(t, h, http.MethodPost, "/v1/workspaces/{id}/members", "/v1/workspaces/W1/members", `{}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewJsonApiCommandHandlerCustomSuccessStatus(t *testing.T) {
	// Command on existing resource (POST /orders/{id}/cancel) returns
	// the updated order with 200, not a "newly created" 201.
	cmdFn := func(_ context.Context, _ dummyInviteCommand) (*dummyMember, error) {
		return newDummyMember("M1", "W1", "acc-9", "admin"), nil
	}
	decoder := func(_ *http.Request) (dummyInviteCommand, error) {
		return dummyInviteCommand{}, nil
	}
	encoder := func(m *dummyMember) *dummyMember { return m }

	h := rest.NewJsonApiCommandHandler(cmdFn, decoder, encoder,
		rest.HandlerWithSuccessStatus(http.StatusOK))
	rec := invokeCommand(t, h, http.MethodPost, "/x", "/x", `{}`)

	assert.Equal(t, http.StatusOK, rec.Code)
}
