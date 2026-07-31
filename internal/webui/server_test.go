package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/control"
)

type fakeController struct {
	requests []control.Request
	response control.Response
	err      error
}

func (f *fakeController) Do(_ context.Context, request control.Request) (control.Response, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

func TestStatusRequiresWebTokenAndUsesSanitizedControlResponse(t *testing.T) {
	controller := &fakeController{response: control.Response{OK: true, Running: true, Routes: []control.RouteSummary{{Alias: "downloads", LocalEndpoint: "127.0.0.1:4768/r/downloads"}}}}
	server, err := New("web_operator_token_which_is_long_enough_123", controller)
	if err != nil {
		t.Fatal(err)
	}
	unauthenticated := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	writer := httptest.NewRecorder()
	server.ServeHTTP(writer, unauthenticated)
	if writer.Code != http.StatusUnauthorized || len(controller.requests) != 0 {
		t.Fatalf("unauthenticated response = %d, requests = %+v", writer.Code, controller.requests)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	request.Header.Set("Authorization", "Bearer web_operator_token_which_is_long_enough_123")
	writer = httptest.NewRecorder()
	server.ServeHTTP(writer, request)
	if writer.Code != http.StatusOK || len(controller.requests) != 1 || controller.requests[0].Action != "status" {
		t.Fatalf("status response = %d, requests = %+v", writer.Code, controller.requests)
	}
	if writer.Header().Get("Cache-Control") != "no-store" || writer.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing security headers: %v", writer.Header())
	}
	var response control.Response
	if err := json.NewDecoder(writer.Body).Decode(&response); err != nil || !response.OK || len(response.Routes) != 1 {
		t.Fatalf("response = %+v, %v", response, err)
	}
}

func TestRouteActionsAreConstrained(t *testing.T) {
	controller := &fakeController{response: control.Response{OK: true}}
	server, err := New("web_operator_token_which_is_long_enough_123", controller)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/routes/build/health", nil)
	request.Header.Set("Authorization", "Bearer web_operator_token_which_is_long_enough_123")
	writer := httptest.NewRecorder()
	server.ServeHTTP(writer, request)
	if writer.Code != http.StatusOK || len(controller.requests) != 1 || controller.requests[0].Action != "test_route_health" || controller.requests[0].Alias != "build" {
		t.Fatalf("health action = %d %+v", writer.Code, controller.requests)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/routes/build/enabled", strings.NewReader(`{"enabled":false}`))
	request.Header.Set("Authorization", "Bearer web_operator_token_which_is_long_enough_123")
	request.Header.Set("Content-Type", "application/json")
	writer = httptest.NewRecorder()
	server.ServeHTTP(writer, request)
	if writer.Code != http.StatusOK || len(controller.requests) != 2 || controller.requests[1].Action != "set_route_enabled" || controller.requests[1].Enabled {
		t.Fatalf("enabled action = %d %+v", writer.Code, controller.requests)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/routes/build/delete", nil)
	request.Header.Set("Authorization", "Bearer web_operator_token_which_is_long_enough_123")
	writer = httptest.NewRecorder()
	server.ServeHTTP(writer, request)
	if writer.Code != http.StatusNotFound || len(controller.requests) != 2 {
		t.Fatalf("delete action = %d %+v", writer.Code, controller.requests)
	}
}
