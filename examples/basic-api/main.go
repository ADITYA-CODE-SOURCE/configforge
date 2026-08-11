// Command basic-api is a runnable example ConfigForge HTTP API demonstrating
// public routes, authenticated customer routes, admin-only routes, rate-limited
// routes, feature-controlled endpoints, and query/header redaction.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/engine"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/feature"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/middleware"
)

func main() {
	configPath := flag.String("config", "examples/configs/default.yaml", "Path to ConfigForge YAML configuration")
	addr := flag.String("addr", ":8080", "Listen address")
	flag.Parse()

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	runtime, err := engine.Compile(*cfg)
	if err != nil {
		log.Fatalf("compile: %v", err)
	}

	store := middleware.NewMemoryStorage(time.Minute)
	defer store.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/products", productsHandler(runtime))
	mux.HandleFunc("/api/payments/create", paymentsHandler)
	mux.HandleFunc("/api/admin/reports", adminReportsHandler)

	handler := middleware.RequestID(runtime)(mux)
	handler = middleware.Redaction(runtime)(handler)
	handler = middleware.RateLimit(runtime, store, nil)(handler)
	handler = middleware.Security(runtime, nil)(handler)
	handler = middleware.DecisionLog(runtime, nil, nil)(handler)

	log.Printf("ConfigForge example API listening on %s (config=%s)", *addr, *configPath)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func productsHandler(runtime *engine.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := runtime.EvaluateFeature("new_checkout", feature.EvaluationContext{
			UserID:  r.Header.Get("X-User-ID"),
			Country: r.URL.Query().Get("country"),
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"products": []map[string]any{
				{"id": "p1", "name": "Widget", "price": 9.99},
				{"id": "p2", "name": "Gadget", "price": 14.99},
			},
			"new_checkout_enabled": dec.Enabled,
			"new_checkout_reason":  dec.Reason,
		})
	}
}

func paymentsHandler(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":  "created",
		"payload": body,
	})
}

func adminReportsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"reports": []map[string]any{
			{"id": "r1", "type": "sales", "total": 12345.67},
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

var _ = fmt.Sprintf
