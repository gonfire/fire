package cinder

import (
	"context"

	"github.com/opentracing/opentracing-go"
)

// Branch will wrap the provided context with the tail of the found trace in the
// context if the root span matches the span found in the context. This ensures
// that opentracing compatible code will properly branch of the trace tail
// rather than the root.
func Branch(ctx context.Context) context.Context {
	// check context
	if ctx == nil {
		return nil
	}

	// get span
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return ctx
	}

	// get trace
	trace := GetTrace(ctx)
	if trace == nil {
		return ctx
	}

	// wrap context with tail if the found span is the trace root and the tail
	// is not the root
	if trace.Root() == span && trace.Tail() != trace.Root() {
		ctx = opentracing.ContextWithSpan(ctx, trace.Tail())
	}

	return ctx
}
