package serviceregistry

import (
	"context"
	"fmt"
	"net"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
)

type Endpoint struct {
	IPv4 string
	Port int32
}

type ServiceRegistry interface {
	QueryEndpoints(namespace string, serviceName string) ([]Endpoint, error)
}

type KubernetesServiceRegistry struct {
	factory              informers.SharedInformerFactory
	servicesLister       corelisters.ServiceLister
	endpointSlicesLister discoverylisters.EndpointSliceLister
	syncChecks           []cache.InformerSynced
}

func New(ctx context.Context, client kubernetes.Interface, resyncPeriod time.Duration) (ServiceRegistry, error) {
	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	servicesInformer := factory.Core().V1().Services()
	endpointSlicesInformer := factory.Discovery().V1().EndpointSlices()

	registry := &KubernetesServiceRegistry{
		factory:              factory,
		servicesLister:       servicesInformer.Lister(),
		endpointSlicesLister: endpointSlicesInformer.Lister(),
		syncChecks: []cache.InformerSynced{
			servicesInformer.Informer().HasSynced,
			endpointSlicesInformer.Informer().HasSynced,
		},
	}

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), registry.syncChecks...) {
		return nil, fmt.Errorf("failed to sync service registry informers")
	}

	return registry, nil
}

func (r *KubernetesServiceRegistry) QueryEndpoints(namespace string, serviceName string) ([]Endpoint, error) {
	service, err := r.servicesLister.Services(namespace).Get(serviceName)
	if err != nil {
		return nil, fmt.Errorf("get service %s/%s: %w", namespace, serviceName, err)
	}

	endpointSlices, err := r.endpointSlicesLister.EndpointSlices(namespace).List(labels.SelectorFromSet(labels.Set{
		discoveryv1.LabelServiceName: serviceName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list endpoint slices %s/%s: %w", namespace, serviceName, err)
	}

	result := make([]Endpoint, 0)
	seen := map[Endpoint]struct{}{}
	for _, endpointSlice := range endpointSlices {
		resolvedPorts := resolveTargetPorts(service.Spec.Ports, endpointSlice.Ports)
		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}

			for _, address := range endpoint.Addresses {
				ip := net.ParseIP(address)
				if ip == nil || ip.To4() == nil {
					continue
				}

				for _, port := range resolvedPorts {
					candidate := Endpoint{
						IPv4: address,
						Port: port,
					}
					if _, exists := seen[candidate]; exists {
						continue
					}
					seen[candidate] = struct{}{}
					result = append(result, candidate)
				}
			}
		}
	}

	slices.SortFunc(result, func(a Endpoint, b Endpoint) int {
		if a.IPv4 < b.IPv4 {
			return -1
		}
		if a.IPv4 > b.IPv4 {
			return 1
		}
		switch {
		case a.Port < b.Port:
			return -1
		case a.Port > b.Port:
			return 1
		default:
			return 0
		}
	})

	return result, nil
}

func resolveTargetPorts(servicePorts []corev1.ServicePort, endpointSlicePorts []discoveryv1.EndpointPort) []int32 {
	resolved := make([]int32, 0, len(servicePorts))
	seen := map[int32]struct{}{}

	for _, port := range servicePorts {
		var target int32
		switch port.TargetPort.Type {
		case intstr.Int:
			if port.TargetPort.IntValue() == 0 {
				target = port.Port
				break
			}
			target = int32(port.TargetPort.IntValue())
		case intstr.String:
			name := port.TargetPort.String()
			matched, ok := matchNamedEndpointPort(name, endpointSlicePorts)
			if !ok {
				continue
			}
			target = matched
		default:
			target = port.Port
		}

		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		resolved = append(resolved, target)
	}

	slices.Sort(resolved)
	return resolved
}

func matchNamedEndpointPort(name string, endpointSlicePorts []discoveryv1.EndpointPort) (int32, bool) {
	for _, port := range endpointSlicePorts {
		if port.Name == nil || *port.Name != name {
			continue
		}
		if port.Port == nil {
			return 0, false
		}
		return *port.Port, true
	}
	return 0, false
}
