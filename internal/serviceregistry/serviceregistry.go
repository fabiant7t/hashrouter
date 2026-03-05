package serviceregistry

import (
	"context"
	"fmt"
	"net"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
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
	factory         informers.SharedInformerFactory
	servicesLister  corelisters.ServiceLister
	endpointsLister corelisters.EndpointsLister
	syncChecks      []cache.InformerSynced
}

func New(ctx context.Context, client kubernetes.Interface, resyncPeriod time.Duration) (ServiceRegistry, error) {
	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	servicesInformer := factory.Core().V1().Services()
	endpointsInformer := factory.Core().V1().Endpoints()

	registry := &KubernetesServiceRegistry{
		factory:         factory,
		servicesLister:  servicesInformer.Lister(),
		endpointsLister: endpointsInformer.Lister(),
		syncChecks: []cache.InformerSynced{
			servicesInformer.Informer().HasSynced,
			endpointsInformer.Informer().HasSynced,
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

	endpoints, err := r.endpointsLister.Endpoints(namespace).Get(serviceName)
	if err != nil {
		return nil, fmt.Errorf("get endpoints %s/%s: %w", namespace, serviceName, err)
	}

	result := make([]Endpoint, 0)
	for _, subset := range endpoints.Subsets {
		resolvedPorts := resolveTargetPorts(service.Spec.Ports, subset)
		for _, address := range subset.Addresses {
			ip := net.ParseIP(address.IP)
			if ip == nil || ip.To4() == nil {
				continue
			}

			for _, port := range resolvedPorts {
				result = append(result, Endpoint{
					IPv4: address.IP,
					Port: port,
				})
			}
		}
	}

	return result, nil
}

func resolveTargetPorts(servicePorts []corev1.ServicePort, subset corev1.EndpointSubset) []int32 {
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
			matched, ok := matchNamedEndpointPort(name, subset.Ports)
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

func matchNamedEndpointPort(name string, endpointPorts []corev1.EndpointPort) (int32, bool) {
	for _, port := range endpointPorts {
		if port.Name == name {
			return port.Port, true
		}
	}
	return 0, false
}
