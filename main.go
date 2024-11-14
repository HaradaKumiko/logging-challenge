package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

var meter otelmetric.Meter
var reqHistogram otelmetric.Float64Histogram
var tracer oteltrace.Tracer

func main() {

	tracer = otel.Tracer("example")

	ctx := context.Background()
	resourceOtel, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String("logging-challange")))

	if err != nil {
		log.Fatal().Err(err).Msg("unable to create resource otel")
	}

	exporter, err := prometheus.New()

	if err != nil {
		log.Fatal().Err(err).Msg("unable to create exporter")
	}

	metricProvider := metric.NewMeterProvider(
		metric.WithResource(resourceOtel),
		metric.WithReader(exporter),
	)

	otel.SetMeterProvider(metricProvider)

	meter = otel.Meter("logging-challange-code")

	reqHistogram, err = meter.Float64Histogram("http_request_duration_miliseconds",
		otelmetric.WithUnit("miliseconds"),
		otelmetric.WithDescription("service latency"),
		otelmetric.WithExplicitBucketBoundaries([]float64{10, 50, 100, 200, 500, 1000}...))

	if err != nil {
		log.Fatal().Err(err).Msg("unable to create measurement meter")
	}

	tc := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithDialOption(grpc.WithBlock()),
	)

	tracerExporter, err := otlptrace.New(ctx, tc)

	if err != nil {
		log.Fatal().Err(err).Msg("unable to create measurement meter")
	}

	bsp := trace.NewBatchSpanProcessor(tracerExporter)

	tracerProvider := trace.NewTracerProvider(
		trace.WithResource(resourceOtel),
		trace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tracerProvider)

	http.Handle("/", otelhttp.WithRouteTag("/", http.HandlerFunc(handler)))
	http.Handle("/another", otelhttp.WithRouteTag("/another", http.HandlerFunc(anotherHandler)))
	http.Handle("/metrics", otelhttp.WithRouteTag("/metrics", promhttp.Handler()))

	otelHandler := otelhttp.NewHandler(http.DefaultServeMux, "server")

	err = os.MkdirAll("logs", os.ModePerm)
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
	err = http.ListenAndServe(":8080", otelhttp.NewHandler(otelHandler, "/"))
	if err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}

}

func handler(w http.ResponseWriter, r *http.Request) {
	context, span := tracer.Start(r.Context(), "handler function", oteltrace.WithAttributes(
		attribute.Bool("good", true),
		attribute.String("user_email", "farhan@rivaldy.com"),
		attribute.Float64("balance", 500_000_000),
	))

	defer span.End()

	startElapsedTime := time.Now()
	if r.URL.Path == "/favicon.ico" {
		http.NotFound(w, r)
		return
	}

	log := log.With().
		Str("trace_id", uuid.NewString()).
		Logger()

	context = log.WithContext(context)

	query := r.URL.Query().Get("name")

	span.SetAttributes(
		attribute.String("name", query),
		attribute.String("location", "Indonesia"),
		attribute.String("environment", "Staging"),
	)

	log.Info().Ctx(context).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("host", r.Host).
		Str("query", query).
		Msg("request received")
	doFirst(context)

	span.AddEvent("handler event", oteltrace.WithAttributes(
		attribute.Bool("successfully write a logging", true),
		attribute.String("host", r.Host),
	))

	if len(query) < 2 {
		var warn = errors.New("NAME TOO SHORT")
		log.Warn().Msg("Name is too short")
		span.SetStatus(codes.Error, warn.Error())
		span.RecordError(warn)
		fmt.Fprintf(w, "Hello, %q! your name is too shoexecutedrt", html.EscapeString(query))
	} else {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(query))
	}

	endElapsedTime := time.Since(startElapsedTime)
	elapsed_ms := float64(endElapsedTime.Nanoseconds() / 10000000)

	log.Info().Ctx(context).
		Float64("elapsed_ms", elapsed_ms).
		Msg("request processed")

	reqHistogram.Record(context, elapsed_ms, otelmetric.WithAttributes(
		attribute.String("url", r.URL.String()),
	))
}

func anotherHandler(w http.ResponseWriter, r *http.Request) {
	context, span := tracer.Start(r.Context(), "anotherHandler function")
	defer span.End()

	startElapsedTime := time.Now()
	log := log.With().
		Str("trace_id", uuid.NewString()).
		Logger()

	context = log.WithContext(context)

	query := r.URL.Query().Get("q")
	log.Info().Ctx(context).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("host", r.Host).
		Str("query", query).
		Msg("request received")

	endElapsedTime := time.Since(startElapsedTime)
	elapsed_ms := float64(endElapsedTime.Nanoseconds() / 10000000)

	fmt.Fprintf(w, "another handler")

	log.Info().Ctx(context).
		Float64("elapsed_ms", float64(endElapsedTime.Nanoseconds()/10000000)).
		Msg("request processed")

	reqHistogram.Record(context, elapsed_ms, otelmetric.WithAttributes(
		attribute.String("url", "/handler"),
	))
}

func doFirst(ctx context.Context) {
	_, span := tracer.Start(ctx, "do first function")
	defer span.End()
	defer span.End()

	log := log.Ctx(ctx)
	log.Info().Msg("do first")
	log.Error().Msg("do second error")
	doSecond(ctx)
}

func doSecond(ctx context.Context) {
	_, span := tracer.Start(ctx, "do second function")
	defer span.End()

	log.Ctx(ctx).Info().Msg("do second")
	log.Ctx(ctx).Warn().Msg("do second warn")
}
