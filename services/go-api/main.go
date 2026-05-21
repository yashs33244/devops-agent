package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})
	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"path"})
	startTime = time.Now()
)

type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		requestCount.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", ww.Status())).Inc()
		requestDuration.WithLabelValues(r.URL.Path).Observe(time.Since(start).Seconds())
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(metricsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "go-api",
			"uptime":  time.Since(startTime).String(),
		})
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ready": true})
	})

	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api", func(r chi.Router) {
		r.Get("/items", func(w http.ResponseWriter, r *http.Request) {
			items := []Item{{1, "Widget"}, {2, "Gadget"}, {3, "Doohickey"}}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
		})
		r.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			writeJSON(w, http.StatusOK, Item{1, "Widget - " + id})
		})
		r.Post("/items", func(w http.ResponseWriter, r *http.Request) {
			var body Item
			json.NewDecoder(r.Body).Decode(&body)
			body.ID = 100
			writeJSON(w, http.StatusCreated, body)
		})
	})

	log.Printf("go-api listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
