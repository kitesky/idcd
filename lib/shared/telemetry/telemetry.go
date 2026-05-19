// Package telemetry provides OpenTelemetry trace initialization and middleware for idcd services.
package telemetry

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds OpenTelemetry configuration.
type Config struct {
	ServiceName    string  // e.g. "idcd-api"
	ServiceVersion string  // e.g. "v1.0.0"
	OTLPEndpoint   string  // e.g. "localhost:4317" (gRPC), empty = stdout exporter
	SamplingRate   float64 // 0.0-1.0, default 0.1 for prod
	Enabled        bool    // false disables tracing entirely
}

// Init initializes OpenTelemetry TracerProvider.
// Returns a shutdown function to call on service exit.
func Init(cfg Config) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled {
		log.Println("[telemetry] tracing disabled")
		return func(context.Context) error { return nil }, nil
	}

	noop := func(context.Context) error { return nil }

	// Create resource with service info
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
		),
	)
	if err != nil {
		return noop, fmt.Errorf("telemetry: failed to create resource: %w", err)
	}

	// When OTLPEndpoint is set, send traces to an OTLP Collector over gRPC.
	// Otherwise fall back to stdout (dev / no-collector environments).
	var exporter sdktrace.SpanExporter
	if cfg.OTLPEndpoint != "" {
		conn, err := grpc.NewClient(cfg.OTLPEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return noop, fmt.Errorf("telemetry: grpc dial %s: %w", cfg.OTLPEndpoint, err)
		}
		exporter, err = otlptracegrpc.New(context.Background(), otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return noop, fmt.Errorf("telemetry: OTLP exporter: %w", err)
		}
		log.Printf("[telemetry] OTLP exporter → %s", cfg.OTLPEndpoint)
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return noop, fmt.Errorf("telemetry: stdout exporter: %w", err)
		}
		log.Println("[telemetry] stdout exporter (no OTLPEndpoint configured)")
	}

	// Create sampler (default 0.1 = 10% sampling)
	samplingRate := cfg.SamplingRate
	if samplingRate <= 0 {
		samplingRate = 0.1
	}
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRate))

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Register as global provider
	otel.SetTracerProvider(tp)

	// Set global propagator (for extracting trace context from incoming requests)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("[telemetry] initialized for service=%s version=%s sampling=%.2f", cfg.ServiceName, cfg.ServiceVersion, samplingRate)

	// Return shutdown function
	return func(ctx context.Context) error {
		log.Println("[telemetry] shutting down TracerProvider")
		return tp.Shutdown(ctx)
	}, nil
}

// TraceMiddleware returns a chi middleware that creates spans per request.
func TraceMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request headers
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start span
			spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					// OTel HTTP semconv v1.21+ stable attribute names.
					attribute.String("http.request.method", r.Method),
					attribute.String("url.path", r.URL.Path),
					attribute.String("url.scheme", r.URL.Scheme),
					attribute.String("user_agent.original", r.Header.Get("User-Agent")),
					attribute.String("server.address", r.Host),
				),
			)
			defer span.End()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request with trace context
			start := time.Now()
			next.ServeHTTP(rw, r.WithContext(ctx))
			duration := time.Since(start)

			// Add response attributes
			span.SetAttributes(
				attribute.Int("http.response.status_code", rw.statusCode),
				attribute.Int64("http.response_time_ms", duration.Milliseconds()),
			)

			// Mark span as error if status >= 500
			if rw.statusCode >= 500 {
				span.SetAttributes(attribute.Bool("error", true))
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}
