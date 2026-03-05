package serviceregistry

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
)

type Endpoint struct {
	Addresses  []string
	TargetPort int32
	NodeName   string
}

type ServiceRegistry interface {
	QueryEndpoints(namespace string, serviceName string) ([]Endpoint, error)
}

type KubernetesServiceRegistry struct {
	factory              informers.SharedInformerFactory
	endpointSlicesLister discoverylisters.EndpointSliceLister
	syncChecks           []cache.InformerSynced
}

func New(ctx context.Context, client kubernetes.Interface, resyncPeriod time.Duration) (ServiceRegistry, error) {
	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	endpointSlicesInformer := factory.Discovery().V1().EndpointSlices()

	registry := &KubernetesServiceRegistry{
		factory:              factory,
		endpointSlicesLister: endpointSlicesInformer.Lister(),
		syncChecks: []cache.InformerSynced{
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
	endpointSlices, err := r.endpointSlicesLister.EndpointSlices(namespace).List(labels.SelectorFromSet(labels.Set{
		discoveryv1.LabelServiceName: serviceName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list endpoint slices %s/%s: %w", namespace, serviceName, err)
	}

	result := make([]Endpoint, 0)
	seen := map[string]struct{}{}
	for _, endpointSlice := range endpointSlices {
		resolvedPorts := resolveTargetPorts(endpointSlice.Ports)
		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}
			if len(endpoint.Addresses) == 0 {
				continue
			}

			nodeName := ""
			if endpoint.NodeName != nil {
				nodeName = *endpoint.NodeName
			}
			addresses := slices.Clone(endpoint.Addresses)
			addressesKey := normalizedAddressesKey(addresses)

			for _, port := range resolvedPorts {
				key := endpointKey(addressesKey, port, nodeName)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				result = append(result, Endpoint{
					Addresses:  addresses,
					TargetPort: port,
					NodeName:   nodeName,
				})
			}
		}
	}

	slices.SortFunc(result, func(a Endpoint, b Endpoint) int {
		aKey := normalizedAddressesKey(a.Addresses)
		bKey := normalizedAddressesKey(b.Addresses)
		if aKey < bKey {
			return -1
		}
		if aKey > bKey {
			return 1
		}
		switch {
		case a.TargetPort < b.TargetPort:
			return -1
		case a.TargetPort > b.TargetPort:
			return 1
		case a.NodeName < b.NodeName:
			return -1
		case a.NodeName > b.NodeName:
			return 1
		default:
			return 0
		}
	})

	return result, nil
}

func resolveTargetPorts(endpointSlicePorts []discoveryv1.EndpointPort) []int32 {
	resolved := make([]int32, 0, len(endpointSlicePorts))
	seen := map[int32]struct{}{}

	for _, port := range endpointSlicePorts {
		if port.Port == nil || *port.Port <= 0 {
			continue
		}
		target := *port.Port

		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		resolved = append(resolved, target)
	}

	slices.Sort(resolved)
	return resolved
}

func endpointKey(addressesKey string, targetPort int32, nodeName string) string {
	return fmt.Sprintf("%s|%d|%s", addressesKey, targetPort, nodeName)
}

func normalizedAddressesKey(addresses []string) string {
	sorted := slices.Clone(addresses)
	slices.Sort(sorted)
	return strings.Join(sorted, ",")
}
