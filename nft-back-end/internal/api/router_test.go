package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouterAllowsConfiguredCORSOrigin(t *testing.T) {
	router := NewRouter(HandlerServices{}, RouterConfig{
		CORSAllowedOrigins: []string{"http://127.0.0.1:5173"},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("Origin", "http://127.0.0.1:5173")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "http://127.0.0.1:5173", response.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Origin", response.Header().Get("Vary"))
}

func TestRouterHandlesCORSPreflight(t *testing.T) {
	router := NewRouter(HandlerServices{}, RouterConfig{
		CORSAllowedOrigins: []string{"http://127.0.0.1:5173"},
	})
	request := httptest.NewRequest(http.MethodOptions, "/api/auctions", nil)
	request.Header.Set("Origin", "http://127.0.0.1:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "http://127.0.0.1:5173", response.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, response.Header().Get("Access-Control-Allow-Methods"), http.MethodGet)
}
