package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/fabiant7t/hashrouter/internal/rendezvous"
	"github.com/fabiant7t/hashrouter/internal/serviceregistry"
)

const (
	indexPrefix = "hashrouter "
)

type healthResponse struct {
	Health string `json:"health"`
}

type Server struct {
	serviceRegistry serviceregistry.ServiceRegistry
	version         string
	mux             *http.ServeMux
}

func New(serviceRegistry serviceregistry.ServiceRegistry, version string) *Server {
	s := &Server{
		serviceRegistry: serviceRegistry,
		version:         version,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/", s.rootHandler)
	s.mux = mux

	return s
}

func NewHandler(serviceRegistry serviceregistry.ServiceRegistry, version string) http.Handler {
	return New(serviceRegistry, version).Handler()
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(indexPrefix + s.version))
		return
	}

	namespace, serviceName, path, ok := parseServicePath(r.URL.Path)
	if ok {
		s.handleServicePath(w, r, namespace, serviceName, path)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleServicePath(w http.ResponseWriter, r *http.Request, namespace string, serviceName string, path string) {
	if s.serviceRegistry == nil {
		http.Error(w, "service registry unavailable", http.StatusBadGateway)
		return
	}

	endpoints, err := s.serviceRegistry.QueryEndpoints(namespace, serviceName)
	if err != nil {
		http.Error(w, "service endpoints unavailable", http.StatusBadGateway)
		return
	}
	if len(endpoints) == 0 {
		http.Error(w, "service has no endpoints", http.StatusBadGateway)
		return
	}

	candidates := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		candidates = append(candidates, addressesCandidate(endpoint.Addresses))
	}

	_, selectedCandidate := rendezvous.HighestScore(candidates, path)
	targetEndpoint, found := findEndpointByCandidate(endpoints, selectedCandidate)
	if !found {
		http.Error(w, "failed to select service endpoint", http.StatusBadGateway)
		return
	}
	targetAddress, ok := firstAddress(targetEndpoint.Addresses)
	if !ok {
		http.Error(w, "service endpoint has no addresses", http.StatusBadGateway)
		return
	}

	target := fmt.Sprintf("http://%s:%d/%s", targetAddress, targetEndpoint.TargetPort, path)
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func (s *Server) healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(healthResponse{
		Health: "ok",
	})
}

func parseServicePath(path string) (namespace string, serviceName string, remainingPath string, ok bool) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", "", "", false
	}

	segments := strings.Split(trimmed, "/")
	if len(segments) < 3 {
		return "", "", "", false
	}

	return segments[0], segments[1], strings.Join(segments[2:], "/"), true
}

func findEndpointByCandidate(endpoints []serviceregistry.Endpoint, candidate string) (serviceregistry.Endpoint, bool) {
	for _, endpoint := range endpoints {
		if addressesCandidate(endpoint.Addresses) == candidate {
			return endpoint, true
		}
	}
	return serviceregistry.Endpoint{}, false
}

func addressesCandidate(addresses []string) string {
	sorted := slices.Clone(addresses)
	slices.Sort(sorted)
	return strings.Join(sorted, ",")
}

func firstAddress(addresses []string) (string, bool) {
	if len(addresses) == 0 {
		return "", false
	}
	sorted := slices.Clone(addresses)
	slices.Sort(sorted)
	return sorted[0], true
}
