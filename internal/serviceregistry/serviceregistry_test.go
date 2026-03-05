package serviceregistry_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/fabiant7t/hashrouter/internal/serviceregistry"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestQueryEndpoints_FromEndpointSlicePorts(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-1",
				Namespace: "default",
				Labels: map[string]string{
					discoveryv1.LabelServiceName: "api",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.10"},
					NodeName:  strPtr("node-a"),
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Port: int32Ptr(8443),
				},
			},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	registry, err := serviceregistry.New(ctx, client, 0)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	got, err := registry.QueryEndpoints("default", "api")
	if err != nil {
		t.Fatalf("query endpoints: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("endpoint count mismatch: got %d want %d", len(got), 1)
	}

	if !slices.Equal(got[0].Addresses, []string{"10.0.0.10"}) || got[0].TargetPort != 8443 || got[0].NodeName != "node-a" {
		t.Fatalf("endpoint mismatch: got %+v", got[0])
	}
}

func TestQueryEndpoints_WithMultipleAddressesAndNodeName(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "web-1",
				Namespace: "default",
				Labels: map[string]string{
					discoveryv1.LabelServiceName: "web",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.20", "2001:db8::1"},
					NodeName:  strPtr("node-b"),
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Port: int32Ptr(8080),
				},
			},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	registry, err := serviceregistry.New(ctx, client, 0)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	got, err := registry.QueryEndpoints("default", "web")
	if err != nil {
		t.Fatalf("query endpoints: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("endpoint count mismatch: got %d want %d", len(got), 1)
	}

	if !slices.Equal(got[0].Addresses, []string{"10.0.0.20", "2001:db8::1"}) || got[0].TargetPort != 8080 || got[0].NodeName != "node-b" {
		t.Fatalf("endpoint mismatch: got %+v", got[0])
	}
}

func TestQueryEndpoints_NotFound_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	registry, err := serviceregistry.New(ctx, client, 0)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	got, err := registry.QueryEndpoints("default", "missing")
	if err != nil {
		t.Fatalf("query endpoints: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("endpoint count mismatch: got %d want %d", len(got), 0)
	}
}

func TestQueryEndpoints_MultipleEndpointSlices_DeduplicatesAndFiltersNotReady(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "web-1",
				Namespace: "default",
				Labels: map[string]string{
					discoveryv1.LabelServiceName: "web",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.20"},
					NodeName:  strPtr("node-a"),
				},
				{
					Addresses: []string{"10.0.0.21"},
					NodeName:  strPtr("node-b"),
					Conditions: discoveryv1.EndpointConditions{
						Ready: boolPtr(false),
					},
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Port: int32Ptr(8080),
				},
			},
		},
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "web-2",
				Namespace: "default",
				Labels: map[string]string{
					discoveryv1.LabelServiceName: "web",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.20"},
					NodeName:  strPtr("node-a"),
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Port: int32Ptr(8080),
				},
			},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	registry, err := serviceregistry.New(ctx, client, 0)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	got, err := registry.QueryEndpoints("default", "web")
	if err != nil {
		t.Fatalf("query endpoints: %v", err)
	}

	want := []serviceregistry.Endpoint{
		{Addresses: []string{"10.0.0.20"}, TargetPort: 8080, NodeName: "node-a"},
	}
	if len(got) != len(want) || !slices.Equal(got[0].Addresses, want[0].Addresses) || got[0].TargetPort != want[0].TargetPort || got[0].NodeName != want[0].NodeName {
		t.Fatalf("endpoint mismatch: got %+v want %+v", got, want)
	}
}

func TestQueryEndpoints_DeduplicatesWhenAddressOrderDiffers(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-1",
				Namespace: "default",
				Labels: map[string]string{
					discoveryv1.LabelServiceName: "api",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.10", "10.0.0.11"},
					NodeName:  strPtr("node-a"),
				},
			},
			Ports: []discoveryv1.EndpointPort{{Port: int32Ptr(8443)}},
		},
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-2",
				Namespace: "default",
				Labels: map[string]string{
					discoveryv1.LabelServiceName: "api",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.11", "10.0.0.10"},
					NodeName:  strPtr("node-a"),
				},
			},
			Ports: []discoveryv1.EndpointPort{{Port: int32Ptr(8443)}},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	registry, err := serviceregistry.New(ctx, client, 0)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	got, err := registry.QueryEndpoints("default", "api")
	if err != nil {
		t.Fatalf("query endpoints: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("endpoint count mismatch: got %d want %d", len(got), 1)
	}
}

func strPtr(value string) *string {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
