package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fabiant7t/hashrouter/internal/rendezvous"
	"github.com/fabiant7t/hashrouter/internal/server"
	"github.com/fabiant7t/hashrouter/internal/serviceregistry"
)

func TestNewHandler_Routes(t *testing.T) {
	t.Parallel()

	mockRegistry := &serviceRegistryMock{
		queryEndpointsFunc: func(namespace string, serviceName string) ([]serviceregistry.Endpoint, error) {
			return []serviceregistry.Endpoint{
				{PrivateIPv4: "10.1.0.3", TargetPort: 8080, NodeName: "node-a"},
				{PrivateIPv4: "10.1.0.4", TargetPort: 8081, NodeName: "node-b"},
			}, nil
		},
	}

	handler := server.NewHandler(mockRegistry, "dev")
	_, selectedIP := rendezvous.HighestScore([]string{"10.1.0.3", "10.1.0.4"}, "v1/users")
	selectedPort := int32(8080)
	if selectedIP == "10.1.0.4" {
		selectedPort = 8081
	}
	expectedLocation := "http://" + selectedIP + ":" + "8080" + "/v1/users"
	if selectedPort == 8081 {
		expectedLocation = "http://" + selectedIP + ":" + "8081" + "/v1/users"
	}

	tests := []struct {
		name       string
		path       string
		wantCode   int
		wantBody   string
		wantHeader string
		wantLoc    string
	}{
		{
			name:       "index",
			path:       "/",
			wantCode:   http.StatusOK,
			wantBody:   "hashrouter dev",
			wantHeader: "text/plain; charset=utf-8",
		},
		{
			name:       "healthz",
			path:       "/healthz",
			wantCode:   http.StatusOK,
			wantBody:   "{\"health\":\"ok\"}\n",
			wantHeader: "application/json",
		},
		{
			name:       "service path",
			path:       "/default/api/v1/users",
			wantCode:   http.StatusTemporaryRedirect,
			wantBody:   "<a href=\"" + expectedLocation + "\">Temporary Redirect</a>.\n\n",
			wantHeader: "text/html; charset=utf-8",
			wantLoc:    expectedLocation,
		},
		{
			name:       "missing service path segment",
			path:       "/default/api",
			wantCode:   http.StatusNotFound,
			wantBody:   "404 page not found\n",
			wantHeader: "text/plain; charset=utf-8",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status code mismatch: got %d want %d", rec.Code, tc.wantCode)
			}

			if rec.Body.String() != tc.wantBody {
				t.Fatalf("body mismatch: got %q want %q", rec.Body.String(), tc.wantBody)
			}

			if got := rec.Header().Get("Content-Type"); got != tc.wantHeader {
				t.Fatalf("content-type mismatch: got %q want %q", got, tc.wantHeader)
			}

			if tc.wantLoc != "" {
				if got := rec.Header().Get("Location"); got != tc.wantLoc {
					t.Fatalf("location mismatch: got %q want %q", got, tc.wantLoc)
				}
			}
		})
	}
}

func TestHealthzSchema(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.NewHandler(nil, "dev").ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unexpected invalid json: %v", err)
	}

	if body["health"] != "ok" {
		t.Fatalf("health value mismatch: got %q want %q", body["health"], "ok")
	}
}

func TestServicePath_NoEndpoints_ReturnsBadGateway(t *testing.T) {
	t.Parallel()

	mockRegistry := &serviceRegistryMock{
		queryEndpointsFunc: func(namespace string, serviceName string) ([]serviceregistry.Endpoint, error) {
			return nil, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/default/api/v1/users", nil)
	rec := httptest.NewRecorder()

	server.NewHandler(mockRegistry, "dev").ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status code mismatch: got %d want %d", rec.Code, http.StatusBadGateway)
	}
}

type serviceRegistryMock struct {
	queryEndpointsFunc func(namespace string, serviceName string) ([]serviceregistry.Endpoint, error)
}

func (m *serviceRegistryMock) QueryEndpoints(namespace string, serviceName string) ([]serviceregistry.Endpoint, error) {
	return m.queryEndpointsFunc(namespace, serviceName)
}
