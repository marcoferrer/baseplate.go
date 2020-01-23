package tracing

import (
	"context"
	"fmt"
	"testing"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"

	"github.com/apache/thrift/lib/go/thrift"
)

func TestStartSpanFromThriftContext(t *testing.T) {
	const (
		name = "foo"

		traceInt = 12345
		traceStr = "12345"

		spanInt = 54321
		spanStr = "54321"
	)

	tracer := Tracer{
		Logger: log.TestWrapper(t),
	}

	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingTrace, traceStr)
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSpan, spanStr)

	ctx, span := StartSpanFromThriftContextWithTracer(ctx, name, &tracer)
	zs := span.trace.toZipkinSpan()

	if zs.TraceID != traceInt {
		t.Errorf(
			"span's traceID expected %d, got %d",
			traceInt,
			zs.TraceID,
		)
	}

	if zs.ParentID != spanInt {
		t.Errorf(
			"span's parent id expected %d, got %d",
			spanInt,
			zs.ParentID,
		)
	}
}

func TestCreateThriftContextFromSpan(t *testing.T) {
	const (
		name    = "foo"
		traceID = "12345"
		spanID  = "54321"
	)

	checkContextKey := func(t *testing.T, ctx context.Context, key string) {
		t.Helper()

		headersSet := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))
		if !headersSet.Contains(key) {
			t.Errorf("context should have %s", key)
		}
	}

	parentCtx := context.Background()
	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingTrace, traceID)
	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingSpan, spanID)

	tracer := Tracer{
		Logger: log.TestWrapper(t),
	}
	_, span := StartSpanFromThriftContextWithTracer(parentCtx, name, &tracer)

	t.Run(
		"not-sampled-and-new",
		func(t *testing.T) {
			ctx := context.Background()
			child := span.CreateClientChild("test")
			ctx = CreateThriftContextFromSpan(ctx, child)
			checkContextKey(t, ctx, thriftbp.HeaderTracingTrace)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); !ok || v != traceID {
				t.Errorf(
					"trace in the context expected to be %q, got %q & %v",
					traceID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingParent)
			expectedParentID := fmt.Sprintf("%v", child.trace.parentID)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingParent); !ok || v != expectedParentID {
				t.Errorf(
					"parent in the context expected to be %q, got %q & %v",
					expectedParentID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSpan)
			expectedSpanID := fmt.Sprintf("%d", child.trace.toZipkinSpan().SpanID)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); !ok || v != expectedSpanID {
				t.Errorf(
					"span in the context expected to be %q, got %q & %v",
					expectedSpanID,
					v,
					ok,
				)
			}

			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled); ok {
				t.Errorf(
					"sampled should not be in the context, got %q & %v",
					v,
					ok,
				)
			}
		},
	)

	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	_, span = StartSpanFromThriftContextWithTracer(parentCtx, name, &tracer)

	t.Run(
		"sampled-and-overwrite",
		func(t *testing.T) {
			ctx := context.Background()
			ctx = thrift.SetWriteHeaderList(ctx, thriftbp.HeadersToForward)
			child := span.CreateClientChild("test")
			ctx = CreateThriftContextFromSpan(ctx, child)

			headers := thrift.GetWriteHeaderList(ctx)
			headersSet := set.StringSliceToSet(headers)
			if len(headers) != len(headersSet) {
				t.Errorf(
					"Expected no duplications in write header list, got %+v",
					headers,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingTrace)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); !ok || v != traceID {
				t.Errorf(
					"trace in the context expected to be %q, got %q & %v",
					traceID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingParent)
			expectedParentID := fmt.Sprintf("%v", child.trace.parentID)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingParent); !ok || v != expectedParentID {
				t.Errorf(
					"parent in the context expected to be %q, got %q & %v",
					expectedParentID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSpan)
			expectedSpanID := fmt.Sprintf("%d", child.trace.toZipkinSpan().SpanID)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); !ok || v != expectedSpanID {
				t.Errorf(
					"span in the context expected to be %q, got %q & %v",
					expectedSpanID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSampled)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled); !ok || v != thriftbp.HeaderTracingSampledTrue {
				t.Errorf(
					"sampled in the context expected to be %q, got %q & %v",
					thriftbp.HeaderTracingSampledTrue,
					v,
					ok,
				)
			}
		},
	)
}
