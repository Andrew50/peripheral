package metrics

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer handles Prometheus metrics exposure
type MetricsServer struct {
	server *http.Server
	port   string
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(port string) *MetricsServer {
	if port == "" {
		port = ":9090" // Default Prometheus port
	}
	if port[0] != ':' {
		port = ":" + port
	}

	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Add some basic application info
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service": "securities-api", "version": "1.0.0"}`))
	})

	server := &http.Server{
		Addr:    port,
		Handler: mux,
		// Configure timeouts
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &MetricsServer{
		server: server,
		port:   port,
	}
}

// Start begins serving metrics
func (ms *MetricsServer) Start() error {
	log.Printf("Starting metrics server on port %s", ms.port)

	go func() {
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the metrics server
func (ms *MetricsServer) Stop(ctx context.Context) error {
	log.Println("Shutting down metrics server...")
	return ms.server.Shutdown(ctx)
}

// Example usage in your main.go:
/*
func main() {
	// ... your existing setup ...

	// Start metrics server
	metricsServer := metrics.NewMetricsServer("9090")
	if err := metricsServer.Start(); err != nil {
		log.Fatalf("Failed to start metrics server: %v", err)
	}

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := metricsServer.Stop(ctx); err != nil {
		log.Printf("Error stopping metrics server: %v", err)
	}

	// ... rest of your shutdown logic ...
}
*/

// Additional business metrics you might want to track
var (
	// Application-wide metrics
	ActiveUsersGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "securities_active_users",
			Help: "Number of currently active users",
		},
	)

	// Database connection metrics
	DatabaseConnectionsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "securities_db_connections",
			Help: "Number of database connections",
		},
		[]string{"state"}, // active, idle, etc.
	)

	// Cache hit rate (if you're using cache)
	CacheHitRate = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "securities_cache_requests_total",
			Help: "Total cache requests",
		},
		[]string{"result"}, // hit, miss
	)
)

func init() {
	// Register the additional metrics
	prometheus.MustRegister(ActiveUsersGauge)
	prometheus.MustRegister(DatabaseConnectionsGauge)
	prometheus.MustRegister(CacheHitRate)
}
