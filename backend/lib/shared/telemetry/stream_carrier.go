// Package telemetry: StreamCarrier 是 Redis Streams XAdd `values` map 的
// OTel TextMapCarrier 实现, 让 W3C TraceContext 能跨 process 异步边界
// (scheduler → gateway → agent → aggregator), 把原来在 HTTP 同步链上的
// trace_id 续接到 Redis Stream 这条线上, 一个 trace_id 串起整条 pipeline.
//
// 背景见 docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md P1-5:
// "跨服务 trace 在异步边界断了" — HTTP TraceMiddleware 已存在,
// 但 Redis Stream 写入时没把 trace context 编码进 message values,
// 消费端拿到 message 就是个新 trace, 跟原请求 trace 对不上。
//
// 用法 (生产端 / scheduler / api / gateway):
//
//	vals := map[string]any{ /* business fields */ }
//	telemetry.InjectStream(ctx, vals)
//	client.Add(ctx, stream, vals) // values 里多了 W3C traceparent / tracestate
//
// 用法 (消费端 / aggregator / dispatcher):
//
//	for _, msg := range messages {
//	    ctx := telemetry.ExtractStream(parentCtx, msg.Values)
//	    // 现在 ctx 带着上游的 trace_id, 在此 ctx 上 start 的 span 会挂回原 trace
//	}
//
// 字段约定: 注入的 key 完全由 OTel propagator 决定, 默认是
// W3C TraceContext propagator 写入的 "traceparent" 和 "tracestate"
// (Composite 还会写 baggage 的 "baggage" key). 业务字段绝不能用这几个
// 名字 — 业务字段都用 snake_case 业务名, 跟 W3C header 名天然不冲突。
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// StreamCarrier adapts a Redis Stream XAdd values map (map[string]any) to the
// OTel propagation.TextMapCarrier interface. Only string values are read back;
// non-string entries collide with the carrier surface but are simply ignored
// on Get (returning "") so the carrier is panic-free in the face of mixed
// payloads (业务字段大多是 string, 但 int / bool / []byte 偶尔存在).
type StreamCarrier map[string]any

// Get returns the value associated with key as a string. Non-string values
// (e.g. integer task fields) return "" rather than panic — the carrier API
// is text-only by spec, so we treat any non-text value as "not present".
func (c StreamCarrier) Get(key string) string {
	v, ok := c[key]
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// Set stores the (key, value) pair in the underlying map.
// OTel TextMapPropagator implementations only call Set with W3C-spec keys
// like "traceparent" / "tracestate" / "baggage".
func (c StreamCarrier) Set(key, value string) {
	c[key] = value
}

// Keys returns all keys currently in the carrier. OTel uses this when
// propagators want to know what's already present (rare in practice).
// Order is not guaranteed (Go map iteration).
func (c StreamCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// Compile-time assertion: StreamCarrier satisfies propagation.TextMapCarrier.
var _ propagation.TextMapCarrier = (StreamCarrier)(nil)

// InjectStream encodes the trace context from ctx into vals as additional
// stream fields (typically "traceparent" + "tracestate"). Idempotent and
// no-op when ctx carries no active span — never panics, even on a nil
// global propagator (otel.GetTextMapPropagator never returns nil).
//
// Call this just before stream.Client.Add / XAdd so the consumer-side
// ExtractStream pulls the same trace context out.
func InjectStream(ctx context.Context, vals map[string]any) {
	if vals == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, StreamCarrier(vals))
}

// ExtractStream returns a new ctx with the trace context decoded from vals
// merged into the parent ctx. If vals carries no traceparent (legacy producer
// or local-stack message without OTel), the returned ctx == parent ctx.
//
// Subsequent span starts on the returned ctx will be children of the upstream
// (producer-side) span, knitting the cross-process trace together.
func ExtractStream(ctx context.Context, vals map[string]any) context.Context {
	if len(vals) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, StreamCarrier(vals))
}
