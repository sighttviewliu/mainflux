package api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mainflux/mainflux"
	adapter "github.com/mainflux/mainflux/http"
	"github.com/mainflux/mainflux/http/api"
	"github.com/mainflux/mainflux/http/mocks"
	"github.com/stretchr/testify/assert"
)

func newService() mainflux.MessagePublisher {
	pub := mocks.NewPublisher()
	return adapter.New(pub)
}

func newHTTPServer(pub mainflux.MessagePublisher, cc mainflux.ThingsServiceClient) *httptest.Server {
	mux := api.MakeHandler(pub, cc)
	return httptest.NewServer(mux)
}

type testRequest struct {
	client      *http.Client
	method      string
	url         string
	contentType string
	token       string
	body        io.Reader
}

func (tr testRequest) make() (*http.Response, error) {
	req, err := http.NewRequest(tr.method, tr.url, tr.body)
	if err != nil {
		return nil, err
	}
	if tr.token != "" {
		req.Header.Set("Authorization", tr.token)
	}
	if tr.contentType != "" {
		req.Header.Set("Content-Type", tr.contentType)
	}
	return tr.client.Do(req)
}

func TestPublish(t *testing.T) {
	chanID := "1"
	invalidID := "wrong"
	contentType := "application/senml+json"
	token := "auth_token"
	invalidToken := "invalid_token"
	msg := `[{"n":"current","t":-1,"v":1.6}]`
	id, _ := strconv.ParseUint(chanID, 10, 64)
	thingsClient := mocks.NewThingsClient(map[string]uint64{token: id})
	pub := newService()
	ts := newHTTPServer(pub, thingsClient)
	defer ts.Close()

	cases := map[string]struct {
		chanID      string
		msg         string
		contentType string
		auth        string
		status      int
	}{
		"publish message": {
			chanID:      chanID,
			msg:         msg,
			contentType: contentType,
			auth:        token,
			status:      http.StatusAccepted,
		},
		"publish message without authorization token": {
			chanID:      chanID,
			msg:         msg,
			contentType: contentType,
			auth:        "",
			status:      http.StatusForbidden,
		},
		"publish message with invalid authorization token": {
			chanID:      chanID,
			msg:         msg,
			contentType: contentType,
			auth:        invalidToken,
			status:      http.StatusForbidden,
		},
		"publish message without content type": {
			chanID:      chanID,
			msg:         msg,
			contentType: "",
			auth:        token,
			status:      http.StatusAccepted,
		},
		"publish message to wrong channel": {
			chanID:      invalidID,
			msg:         msg,
			contentType: contentType,
			auth:        token,
			status:      http.StatusNotFound,
		},
		"publish message unable to authorize": {
			chanID:      chanID,
			msg:         msg,
			contentType: contentType,
			auth:        mocks.ServiceErrToken,
			status:      http.StatusServiceUnavailable,
		},
	}

	for desc, tc := range cases {
		req := testRequest{
			client:      ts.Client(),
			method:      http.MethodPost,
			url:         fmt.Sprintf("%s/channels/%s/messages", ts.URL, tc.chanID),
			contentType: tc.contentType,
			token:       tc.auth,
			body:        strings.NewReader(tc.msg),
		}
		res, err := req.make()
		assert.Nil(t, err, fmt.Sprintf("%s: unexpected error %s", desc, err))
		assert.Equal(t, tc.status, res.StatusCode, fmt.Sprintf("%s: expected status code %d got %d", desc, tc.status, res.StatusCode))
	}
}
