package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/appraisal-crm/request-service/internal/middleware"
	"github.com/appraisal-crm/request-service/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockService implements service.RequestService for handler-level tests.
type mockService struct {
	mock.Mock
}

func (m *mockService) Create(ctx context.Context, in service.CreateInput) (*domain.Request, error) {
	args := m.Called(ctx, in)
	return reqOrNil(args.Get(0)), args.Error(1)
}

func (m *mockService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	args := m.Called(ctx, id)
	return reqOrNil(args.Get(0)), args.Error(1)
}

func (m *mockService) Update(ctx context.Context, id uuid.UUID, in service.UpdateInput) (*domain.Request, error) {
	args := m.Called(ctx, id, in)
	return reqOrNil(args.Get(0)), args.Error(1)
}

func (m *mockService) ChangeStatus(ctx context.Context, id uuid.UUID, newStatus domain.Status) (*domain.Request, error) {
	args := m.Called(ctx, id, newStatus)
	return reqOrNil(args.Get(0)), args.Error(1)
}

func (m *mockService) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Request, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Request), args.Error(1)
}

func (m *mockService) ListAll(ctx context.Context, limit, offset int) ([]*domain.Request, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Request), args.Error(1)
}

func reqOrNil(v any) *domain.Request {
	if v == nil {
		return nil
	}
	return v.(*domain.Request)
}

// reqOptions describes the simulated authenticated request for a direct handler call.
type reqOptions struct {
	userID    uuid.UUID
	roles     []string
	urlParams map[string]string
}

// newAuthedRequest builds an *http.Request as it would look after Auth + chi routing:
// the user ID and roles are injected via the middleware context helpers, and any
// path params are placed into a chi RouteContext so chi.URLParam works.
func newAuthedRequest(method, target, body string, opt reqOptions) *http.Request {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, target, rdr)
	r.Header.Set("Content-Type", "application/json")

	ctx := r.Context()
	if opt.userID != uuid.Nil {
		ctx = middleware.ContextWithUserID(ctx, opt.userID)
	}
	if opt.roles != nil {
		ctx = middleware.ContextWithRoles(ctx, opt.roles)
	}
	if len(opt.urlParams) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range opt.urlParams {
			rctx.URLParams.Add(k, v)
		}
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	}
	return r.WithContext(ctx)
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var er errorResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	return er.Error
}

func newRequestFixture(clientID uuid.UUID, status domain.Status) *domain.Request {
	return &domain.Request{
		ID:          uuid.New(),
		ClientID:    clientID,
		Email:       "client@example.com",
		PhoneNumber: "+71234567890",
		Status:      status,
	}
}

// --- Create ------------------------------------------------------------------

func TestCreate_Success(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	clientID := uuid.New()
	created := newRequestFixture(clientID, domain.StatusNew)
	svc.On("Create", mock.Anything, mock.Anything).Return(created, nil)

	w := httptest.NewRecorder()
	body := `{"email":"client@example.com","phone_number":"+71234567890"}`
	r := newAuthedRequest(http.MethodPost, "/requests", body, reqOptions{userID: clientID, roles: []string{"client"}})
	h.Create(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got domain.Request
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, domain.StatusNew, got.Status)
	svc.AssertExpectations(t)
}

func TestCreate_NoUserInContext_401(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	w := httptest.NewRecorder()
	body := `{"email":"client@example.com","phone_number":"+71234567890"}`
	r := newAuthedRequest(http.MethodPost, "/requests", body, reqOptions{}) // no userID
	h.Create(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_InvalidJSON_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPost, "/requests", `not json`, reqOptions{userID: uuid.New(), roles: []string{"client"}})
	h.Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_ValidationFailure_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	w := httptest.NewRecorder()
	body := `{"email":"not-an-email","phone_number":"+71234567890"}`
	r := newAuthedRequest(http.MethodPost, "/requests", body, reqOptions{userID: uuid.New(), roles: []string{"client"}})
	h.Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotEmpty(t, decodeError(t, w))
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_ServiceError_500(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	svc.On("Create", mock.Anything, mock.Anything).Return(nil, errors.New("db exploded"))

	w := httptest.NewRecorder()
	body := `{"email":"client@example.com","phone_number":"+71234567890"}`
	r := newAuthedRequest(http.MethodPost, "/requests", body, reqOptions{userID: uuid.New(), roles: []string{"client"}})
	h.Create(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// internal error text must not leak
	assert.NotContains(t, w.Body.String(), "db exploded")
	svc.AssertExpectations(t)
}

// --- GetByID -----------------------------------------------------------------

func TestGetByID_Success(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	req := newRequestFixture(uuid.New(), domain.StatusNew)
	req.ID = id
	svc.On("GetByID", mock.Anything, id).Return(req, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests/"+id.String(), "", reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.GetByID(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestGetByID_InvalidUUID_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests/not-a-uuid", "", reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": "not-a-uuid"},
	})
	h.GetByID(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestGetByID_NotFound_404(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(nil, service.ErrNotFound)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests/"+id.String(), "", reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.GetByID(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestGetByID_ClientOwnRequest_200(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	clientID := uuid.New()
	req := newRequestFixture(clientID, domain.StatusNew)
	req.ID = id
	svc.On("GetByID", mock.Anything, id).Return(req, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests/"+id.String(), "", reqOptions{
		userID: clientID, roles: []string{"client"}, urlParams: map[string]string{"id": id.String()},
	})
	h.GetByID(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestGetByID_ClientForeignRequest_403(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	ownerID := uuid.New()
	req := newRequestFixture(ownerID, domain.StatusNew)
	req.ID = id
	svc.On("GetByID", mock.Anything, id).Return(req, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests/"+id.String(), "", reqOptions{
		userID: uuid.New(), roles: []string{"client"}, urlParams: map[string]string{"id": id.String()},
	})
	h.GetByID(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestGetByID_ServiceError_500(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(nil, errors.New("db exploded"))

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests/"+id.String(), "", reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.GetByID(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "db exploded")
	svc.AssertExpectations(t)
}

// --- Update ------------------------------------------------------------------

func TestUpdate_Success(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	updated := newRequestFixture(uuid.New(), domain.StatusInProgress)
	updated.ID = id
	svc.On("Update", mock.Anything, id, mock.Anything).Return(updated, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String(), `{"address":"New street 5"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.Update(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestUpdate_InvalidUUID_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/bad", `{"address":"x"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": "bad"},
	})
	h.Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdate_InvalidJSON_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String(), `not json`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdate_ValidationFailure_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String(), `{"object_type":"castle"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.Update(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdate_NotFound_404(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("Update", mock.Anything, id, mock.Anything).Return(nil, service.ErrNotFound)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String(), `{"address":"x"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.Update(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestUpdate_Conflict_409(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("Update", mock.Anything, id, mock.Anything).Return(nil, service.ErrConflict)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String(), `{"address":"x"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.Update(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}

func TestUpdate_ServiceError_500(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("Update", mock.Anything, id, mock.Anything).Return(nil, errors.New("db exploded"))

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String(), `{"address":"x"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.Update(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "db exploded")
	svc.AssertExpectations(t)
}

// --- ChangeStatus ------------------------------------------------------------

func TestChangeStatus_Success(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	updated := newRequestFixture(uuid.New(), domain.StatusInProgress)
	updated.ID = id
	svc.On("ChangeStatus", mock.Anything, id, domain.StatusInProgress).Return(updated, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String()+"/status", `{"status":"in_progress"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestChangeStatus_InvalidUUID_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/bad/status", `{"status":"in_progress"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": "bad"},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "ChangeStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestChangeStatus_MissingStatus_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String()+"/status", `{}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "ChangeStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestChangeStatus_InvalidStatusValue_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String()+"/status", `{"status":"invalid_value"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "ChangeStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestChangeStatus_NotFound_404(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("ChangeStatus", mock.Anything, id, domain.StatusInProgress).Return(nil, service.ErrNotFound)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String()+"/status", `{"status":"in_progress"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestChangeStatus_Conflict_409(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("ChangeStatus", mock.Anything, id, domain.StatusInProgress).Return(nil, service.ErrConflict)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String()+"/status", `{"status":"in_progress"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}

func TestChangeStatus_InvalidTransition_422(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("ChangeStatus", mock.Anything, id, domain.StatusClosed).Return(nil, service.ErrInvalidStatusTransition)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String()+"/status", `{"status":"closed"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	svc.AssertExpectations(t)
}

func TestChangeStatus_ServiceError_500(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	id := uuid.New()
	svc.On("ChangeStatus", mock.Anything, id, domain.StatusInProgress).Return(nil, errors.New("db exploded"))

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String()+"/status", `{"status":"in_progress"}`, reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()},
	})
	h.ChangeStatus(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "db exploded")
	svc.AssertExpectations(t)
}

// --- ListByClientID (list) ---------------------------------------------------

func TestList_Client_ReturnsFlatArray(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	clientID := uuid.New()
	svc.On("ListByClientID", mock.Anything, clientID).Return([]*domain.Request{
		newRequestFixture(clientID, domain.StatusNew),
	}, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests", "", reqOptions{userID: clientID, roles: []string{"client"}})
	h.ListByClientID(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var arr []*domain.Request
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	assert.Len(t, arr, 1)
	svc.AssertExpectations(t)
}

func TestList_AppraiserWithClientID_ReturnsFlatArray(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	filterID := uuid.New()
	svc.On("ListByClientID", mock.Anything, filterID).Return([]*domain.Request{}, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests?client_id="+filterID.String(), "", reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"},
	})
	h.ListByClientID(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	// flat array, not an envelope
	var arr []*domain.Request
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	svc.AssertExpectations(t)
}

func TestList_AppraiserBadClientID_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests?client_id=bad", "", reqOptions{
		userID: uuid.New(), roles: []string{"appraiser"},
	})
	h.ListByClientID(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "ListByClientID", mock.Anything, mock.Anything)
}

func TestList_AppraiserListAll_ReturnsEnvelopeWithDefaults(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	// no params -> page 1, limit 20 -> offset 0
	svc.On("ListAll", mock.Anything, 20, 0).Return([]*domain.Request{}, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests", "", reqOptions{userID: uuid.New(), roles: []string{"admin"}})
	h.ListByClientID(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var env listAllResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, 1, env.Page)
	assert.Equal(t, 20, env.Limit)
	svc.AssertExpectations(t)
}

func TestList_ListAll_PaginationWindow(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	// page=3&limit=5 -> offset 10
	svc.On("ListAll", mock.Anything, 5, 10).Return([]*domain.Request{}, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests?page=3&limit=5", "", reqOptions{userID: uuid.New(), roles: []string{"appraiser"}})
	h.ListByClientID(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var env listAllResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, 3, env.Page)
	assert.Equal(t, 5, env.Limit)
	svc.AssertExpectations(t)
}

func TestList_ListAll_ClampsOversizeLimit(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	// limit=101 is out of range -> falls back to default 20, page 1 -> offset 0
	svc.On("ListAll", mock.Anything, 20, 0).Return([]*domain.Request{}, nil)

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests?limit=101", "", reqOptions{userID: uuid.New(), roles: []string{"appraiser"}})
	h.ListByClientID(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var env listAllResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, 20, env.Limit)
	svc.AssertExpectations(t)
}

func TestList_ClientServiceError_500(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)
	clientID := uuid.New()
	svc.On("ListByClientID", mock.Anything, clientID).Return(nil, errors.New("db exploded"))

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodGet, "/requests", "", reqOptions{userID: clientID, roles: []string{"client"}})
	h.ListByClientID(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "db exploded")
	svc.AssertExpectations(t)
}

// --- Health ------------------------------------------------------------------

func TestHealth_OK(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	Health(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}
