package serviceregistry_test

import (
	"context"
	"testing"
	"time"

	"github.com/fabiant7t/hashrouter/internal/serviceregistry"
	corev1 "k8s.io/api/core/v1"
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
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{IP: "10.0.0.10"},
					},
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
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{IP: "10.0.0.20"},
						{IP: "2001:db8::1"},
					},
					Ports: []corev1.EndpointPort{
						{Name: "http", Port: 8080},
					},
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
