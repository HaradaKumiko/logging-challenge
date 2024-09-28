package main

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus"

)

var (
	reqCountProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_request_count",
			Help: "The total number of processed by handler",
	}, []string{"method", "endpoint"} )
)

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/another", anotherHandler)
	http.Handle("/metrics", promhttp.Handler())

	err := os.MkdirAll("logs", os.ModePerm)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to create logs directory")
	}

	lf, err := os.OpenFile(
		"logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to open log file")
	}

	multiWriters := zerolog.MultiLevelWriter(os.Stdout, lf)

	log.Logger = zerolog.New(multiWriters).With().Timestamp().Logger()

	log.Info().Msg("Starting server on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	reqCountProcessed.With(prometheus.Labels{"method": r.Method, "endpoint": r.URL.Path}).Inc()

	startElapsedTime := time.Now()
	if r.URL.Path == "/favicon.ico" {
		http.NotFound(w, r)
		return
	}

	log := log.With().
		Str("trace_id", uuid.NewString()).
		Logger()

	ctx := log.WithContext(r.Context())

	query := r.URL.Query().Get("q")
	log.Info().Ctx(ctx).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("host", r.Host).
		Str("query", query).
		Msg("request received")
	doFirst(ctx)
	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))

	endElapsedTime := time.Since(startElapsedTime)

	log.Info().Ctx(ctx).
		Float64("elapsed_ms", float64(endElapsedTime.Nanoseconds()/10000000)).
		Msg("request processed")

}

func anotherHandler(w http.ResponseWriter, r *http.Request){
	reqCountProcessed.With(prometheus.Labels{"method": r.Method, "endpoint": r.URL.Path}).Inc()
	startElapsedTime := time.Now()
	log := log.With().
		Str("trace_id", uuid.NewString()).
		Logger()

	ctx := log.WithContext(r.Context())

	query := r.URL.Query().Get("q")
	log.Info().Ctx(ctx).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("host", r.Host).
		Str("query", query).
		Msg("request received")

	endElapsedTime := time.Since(startElapsedTime)

	fmt.Fprintf(w, "another handler")
	log.Info().Ctx(ctx).
		Float64("elapsed_ms", float64(endElapsedTime.Nanoseconds()/10000000)).
		Msg("request processed")
}

func doFirst(ctx context.Context) {
	log := log.Ctx(ctx)
	log.Info().Msg("do first")
	log.Error().Msg("do second error")
	doSecond(ctx)
}

func doSecond(ctx context.Context) {
	log.Ctx(ctx).Info().Msg("do second")
	log.Ctx(ctx).Warn().Msg("do second warn")
}
