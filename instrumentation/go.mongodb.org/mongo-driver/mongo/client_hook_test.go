// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodb

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.opentelemetry.io/otelc/pkg/hook/hooktest"
)

func TestMongoEnabler(t *testing.T) {
	tests := []struct {
		name     string
		setupEnv func(t *testing.T)
		expected bool
	}{
		{
			name: "enabled explicitly",
			setupEnv: func(t *testing.T) {
				t.Setenv("OTEL_GO_ENABLED_INSTRUMENTATIONS", "mongodb")
			},
			expected: true,
		},
		{
			name: "disabled explicitly",
			setupEnv: func(t *testing.T) {
				t.Setenv("OTEL_GO_DISABLED_INSTRUMENTATIONS", "mongodb")
			},
			expected: false,
		},
		{
			name: "not in enabled list",
			setupEnv: func(t *testing.T) {
				t.Setenv("OTEL_GO_ENABLED_INSTRUMENTATIONS", "redis,grpc")
			},
			expected: false,
		},
		{
			name:     "default enabled when no env set",
			setupEnv: func(t *testing.T) {},
			expected: true,
		},
		{
			name: "enabled with multiple instrumentations",
			setupEnv: func(t *testing.T) {
				t.Setenv("OTEL_GO_ENABLED_INSTRUMENTATIONS", "nethttp,mongodb,grpc")
			},
			expected: true,
		},
		{
			name: "disabled with multiple instrumentations",
			setupEnv: func(t *testing.T) {
				t.Setenv("OTEL_GO_DISABLED_INSTRUMENTATIONS", "mongodb,grpc")
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)

			enabler := mongoEnabler{}
			if got := enabler.Enable(); got != tt.expected {
				t.Fatalf("mongoEnabler.Enable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBeforeConnect_Disabled(t *testing.T) {
	t.Setenv("OTEL_GO_DISABLED_INSTRUMENTATIONS", "mongodb")

	ctx := context.Background()
	ictx := hooktest.NewMockHookContext(ctx)

	BeforeConnect(ictx, ctx)

	if got := ictx.GetParamCount(); got != 1 {
		t.Fatalf("GetParamCount() = %d, want 1", got)
	}
	if got := ictx.GetParam(1); got != nil {
		t.Fatalf("GetParam(1) = %#v, want nil", got)
	}
}

func TestBeforeConnect(t *testing.T) {
	tests := []struct {
		name         string
		opts         []*options.ClientOptions
		checkUpdated func(t *testing.T, opts []*options.ClientOptions, originalMonitors []*event.CommandMonitor)
	}{
		{
			name: "creates default options when none provided",
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, _ []*event.CommandMonitor) {
				if len(opts) != 1 {
					t.Fatalf("len(opts) = %d, want 1", len(opts))
				}
				if opts[0] == nil {
					t.Fatal("opts[0] = nil, want non-nil")
				}
				if opts[0].Monitor == nil {
					t.Fatal("opts[0].Monitor = nil, want injected monitor")
				}
			},
		},
		{
			name: "injects monitor into option with nil monitor",
			opts: []*options.ClientOptions{options.Client()},
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, _ []*event.CommandMonitor) {
				if len(opts) != 1 {
					t.Fatalf("len(opts) = %d, want 1", len(opts))
				}
				if opts[0].Monitor == nil {
					t.Fatal("opts[0].Monitor = nil, want injected monitor")
				}
			},
		},
		{
			name: "does not overwrite existing monitor",
			opts: []*options.ClientOptions{options.Client().SetMonitor(&event.CommandMonitor{})},
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, originalMonitors []*event.CommandMonitor) {
				if originalMonitors[0] == nil {
					t.Fatal("opts[0].Monitor = nil, want existing monitor")
				}
				if opts[0].Monitor != originalMonitors[0] {
					t.Fatal("existing monitor was overwritten")
				}
			},
		},
		{
			name: "preserves existing monitor and injects missing ones",
			opts: []*options.ClientOptions{
				options.Client().SetMonitor(&event.CommandMonitor{}),
				options.Client(),
			},
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, originalMonitors []*event.CommandMonitor) {
				if len(opts) != 2 {
					t.Fatalf("len(opts) = %d, want 2", len(opts))
				}
				if opts[0].Monitor != originalMonitors[0] {
					t.Fatal("opts[0].Monitor was overwritten, want preserved monitor")
				}
				if opts[0].Monitor == nil {
					t.Fatal("opts[0].Monitor = nil, want preserved monitor")
				}
				if opts[1].Monitor == nil {
					t.Fatal("opts[1].Monitor = nil, want injected monitor")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_GO_ENABLED_INSTRUMENTATIONS", "mongodb")

			ctx := context.Background()
			originalMonitors := monitors(tt.opts)
			originalOpts := append([]*options.ClientOptions(nil), tt.opts...)
			ictx := hooktest.NewMockHookContext(ctx, originalOpts)

			BeforeConnect(ictx, ctx, tt.opts...)

			updatedOpts, ok := ictx.GetParam(1).([]*options.ClientOptions)
			if !ok {
				t.Fatalf("GetParam(1) type = %T, want []*options.ClientOptions", ictx.GetParam(1))
			}

			tt.checkUpdated(t, updatedOpts, originalMonitors)
		})
	}
}

func TestBeforeNewClient_Disabled(t *testing.T) {
	t.Setenv("OTEL_GO_DISABLED_INSTRUMENTATIONS", "mongodb")

	ictx := hooktest.NewMockHookContext()

	BeforeNewClient(ictx)

	if got := ictx.GetParamCount(); got != 0 {
		t.Fatalf("GetParamCount() = %d, want 0", got)
	}
	if got := ictx.GetParam(0); got != nil {
		t.Fatalf("GetParam(0) = %#v, want nil", got)
	}
}

func TestBeforeNewClient(t *testing.T) {
	tests := []struct {
		name         string
		opts         []*options.ClientOptions
		checkUpdated func(t *testing.T, opts []*options.ClientOptions, originalMonitors []*event.CommandMonitor)
	}{
		{
			name: "creates default options when none provided",
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, _ []*event.CommandMonitor) {
				if len(opts) != 1 {
					t.Fatalf("len(opts) = %d, want 1", len(opts))
				}
				if opts[0] == nil {
					t.Fatal("opts[0] = nil, want non-nil")
				}
				if opts[0].Monitor == nil {
					t.Fatal("opts[0].Monitor = nil, want injected monitor")
				}
			},
		},
		{
			name: "injects monitor into option with nil monitor",
			opts: []*options.ClientOptions{options.Client()},
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, _ []*event.CommandMonitor) {
				if len(opts) != 1 {
					t.Fatalf("len(opts) = %d, want 1", len(opts))
				}
				if opts[0].Monitor == nil {
					t.Fatal("opts[0].Monitor = nil, want injected monitor")
				}
			},
		},
		{
			name: "does not overwrite existing monitor",
			opts: []*options.ClientOptions{options.Client().SetMonitor(&event.CommandMonitor{})},
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, originalMonitors []*event.CommandMonitor) {
				if originalMonitors[0] == nil {
					t.Fatal("opts[0].Monitor = nil, want existing monitor")
				}
				if opts[0].Monitor != originalMonitors[0] {
					t.Fatal("existing monitor was overwritten")
				}
			},
		},
		{
			name: "preserves existing monitor and injects missing ones",
			opts: []*options.ClientOptions{
				options.Client().SetMonitor(&event.CommandMonitor{}),
				options.Client(),
			},
			checkUpdated: func(t *testing.T, opts []*options.ClientOptions, originalMonitors []*event.CommandMonitor) {
				if len(opts) != 2 {
					t.Fatalf("len(opts) = %d, want 2", len(opts))
				}
				if opts[0].Monitor != originalMonitors[0] {
					t.Fatal("opts[0].Monitor was overwritten, want preserved monitor")
				}
				if opts[0].Monitor == nil {
					t.Fatal("opts[0].Monitor = nil, want preserved monitor")
				}
				if opts[1].Monitor == nil {
					t.Fatal("opts[1].Monitor = nil, want injected monitor")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_GO_ENABLED_INSTRUMENTATIONS", "mongodb")

			originalMonitors := monitors(tt.opts)
			originalOpts := append([]*options.ClientOptions(nil), tt.opts...)
			ictx := hooktest.NewMockHookContext(originalOpts)

			BeforeNewClient(ictx, tt.opts...)

			updatedOpts, ok := ictx.GetParam(0).([]*options.ClientOptions)
			if !ok {
				t.Fatalf("GetParam(0) type = %T, want []*options.ClientOptions", ictx.GetParam(0))
			}

			tt.checkUpdated(t, updatedOpts, originalMonitors)
		})
	}
}

func monitors(opts []*options.ClientOptions) []*event.CommandMonitor {
	ms := make([]*event.CommandMonitor, len(opts))
	for i, opt := range opts {
		if opt != nil {
			ms[i] = opt.Monitor
		}
	}
	return ms
}
