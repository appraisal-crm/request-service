package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/appraisal-crm/request-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Input-validation matrix from ACRM-77: invalid input never reaches the service layer.

const validEmail = "client@example.com"
const validPhone = "+79161234567"

// --- POST /requests validation (ACRM-77 TC-VAL-01..15) -----------------------

func TestCreate_InputValidation(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantCall   bool // whether the service Create is expected to be invoked
	}{
		{"TC-VAL-01 missing email", `{"phone_number":"` + validPhone + `"}`, http.StatusBadRequest, false},
		{"TC-VAL-02 missing phone", `{"email":"` + validEmail + `"}`, http.StatusBadRequest, false},
		{"TC-VAL-03 only required fields", `{"email":"` + validEmail + `","phone_number":"` + validPhone + `"}`, http.StatusCreated, true},
		{"TC-VAL-04 empty body", `{}`, http.StatusBadRequest, false},
		{"TC-VAL-05 malformed email", `{"email":"not-an-email","phone_number":"` + validPhone + `"}`, http.StatusBadRequest, false},
		{"TC-VAL-06 empty email", `{"email":"","phone_number":"` + validPhone + `"}`, http.StatusBadRequest, false},
		// TC-VAL-07: the `e164` tag treats the leading "+" as optional — strict E.164
		// is deliberately not enforced until a phone-dependent feature lands
		{"TC-VAL-07 phone missing plus is accepted (documents current behavior)", `{"email":"` + validEmail + `","phone_number":"89161234567"}`, http.StatusCreated, true},
		{"TC-VAL-08 phone too short", `{"email":"` + validEmail + `","phone_number":"+7"}`, http.StatusBadRequest, false},
		{"TC-VAL-09 valid phone", `{"email":"` + validEmail + `","phone_number":"` + validPhone + `"}`, http.StatusCreated, true},
		{"TC-VAL-10 object_type not in enum", `{"email":"` + validEmail + `","phone_number":"` + validPhone + `","object_type":"castle"}`, http.StatusBadRequest, false},
		{"TC-VAL-11 empty address", `{"email":"` + validEmail + `","phone_number":"` + validPhone + `","address":""}`, http.StatusBadRequest, false},
		{"TC-VAL-12 unknown field ignored", `{"email":"` + validEmail + `","phone_number":"` + validPhone + `","title":"ignored"}`, http.StatusCreated, true},
		{"TC-VAL-13 not JSON", `not json`, http.StatusBadRequest, false},
		{"TC-VAL-14 no body", ``, http.StatusBadRequest, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockService{}
			h := newRequestHandler(svc)
			clientID := uuid.New()
			if tc.wantCall {
				svc.On("Create", mock.Anything, mock.Anything).
					Return(newRequestFixture(clientID, domain.StatusNew), nil)
			}

			w := httptest.NewRecorder()
			r := newAuthedRequest(http.MethodPost, "/requests", tc.body,
				reqOptions{userID: clientID, roles: []string{"client"}})
			h.Create(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantCall {
				svc.AssertExpectations(t)
			} else {
				svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
				assert.NotEmpty(t, decodeError(t, w), "error response must carry a readable message")
			}
		})
	}
}

// TC-VAL-15: a body over the 1 MB limit is rejected with 400 before the service.
func TestCreate_BodyTooLarge_400(t *testing.T) {
	svc := &mockService{}
	h := newRequestHandler(svc)

	oversized := `{"email":"` + validEmail + `","phone_number":"` + validPhone +
		`","address":"` + strings.Repeat("a", 1<<20) + `"}`

	w := httptest.NewRecorder()
	r := newAuthedRequest(http.MethodPost, "/requests", oversized,
		reqOptions{userID: uuid.New(), roles: []string{"client"}})
	h.Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// --- PATCH /requests/{id} validation (ACRM-77 TC-VAL-16..19) -----------------

func TestUpdate_InputValidation(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantCall   bool
	}{
		{"TC-VAL-16 empty patch bumps updated_at", `{}`, http.StatusOK, true},
		{"TC-VAL-17 object_type not in enum", `{"object_type":"castle"}`, http.StatusBadRequest, false},
		{"TC-VAL-18 inspector_id not a UUID", `{"inspector_id":"not-a-uuid"}`, http.StatusBadRequest, false},
		{"TC-VAL-19 empty address", `{"address":""}`, http.StatusBadRequest, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockService{}
			h := newRequestHandler(svc)
			id := uuid.New()
			if tc.wantCall {
				updated := newRequestFixture(uuid.New(), domain.StatusInProgress)
				updated.ID = id
				svc.On("Update", mock.Anything, id, mock.Anything).Return(updated, nil)
			}

			w := httptest.NewRecorder()
			r := newAuthedRequest(http.MethodPatch, "/requests/"+id.String(), tc.body,
				reqOptions{userID: uuid.New(), roles: []string{"appraiser"}, urlParams: map[string]string{"id": id.String()}})
			h.Update(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantCall {
				svc.AssertExpectations(t)
			} else {
				svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
				assert.NotEmpty(t, decodeError(t, w), "error response must carry a readable message")
			}
		})
	}
}
