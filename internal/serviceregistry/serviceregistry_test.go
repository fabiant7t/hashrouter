package serviceregistry_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/fabiant7t/hashrouter/internal/serviceregistry"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestQueryEndpoints_WithNumericTargetPort(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{TargetPort: intstr.FromInt32(8443)},
				},
			},
		},
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

	if got[0].IPv4 != "10.0.0.10" || got[0].Port != 8443 {
		t.Fatalf("endpoint mismatch: got %+v", got[0])
	}
}

func TestQueryEndpoints_WithNamedTargetPortAndIPv4Filtering(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{TargetPort: intstr.FromString("http")},
				},
			},
		},
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
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Name: strPtr("http"),
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

	if got[0].IPv4 != "10.0.0.20" || got[0].Port != 8080 {
		t.Fatalf("endpoint mismatch: got %+v", got[0])
	}
}

func TestQueryEndpoints_NotFound(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	registry, err := serviceregistry.New(ctx, client, 0)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	if _, err := registry.QueryEndpoints("default", "missing"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestQueryEndpoints_MultipleEndpointSlices_DeduplicatesAndFiltersNotReady(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{TargetPort: intstr.FromString("http")},
				},
			},
		},
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
				},
				{
					Addresses: []string{"10.0.0.21"},
					Conditions: discoveryv1.EndpointConditions{
						Ready: boolPtr(false),
					},
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Name: strPtr("http"),
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
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Name: strPtr("http"),
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
		{IPv4: "10.0.0.20", Port: 8080},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("endpoint mismatch: got %+v want %+v", got, want)
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
