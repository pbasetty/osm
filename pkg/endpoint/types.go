package endpoint

import (
	"fmt"
	"net"

	"github.com/openservicemesh/osm/pkg/announcements"
	"github.com/openservicemesh/osm/pkg/service"
)

// Provider is an interface to be implemented by components abstracting Kubernetes, and other compute/cluster providers
type Provider interface {
	// Retrieve the IP addresses comprising the given service.
	ListEndpointsForService(service.MeshService) []Endpoint

	// Retrieve the namespaced services for a given service account
	GetServicesForServiceAccount(service.K8sServiceAccount) ([]service.MeshService, error)

	// GetPortToProtocolMappingForService returns a mapping of the service's ports to their corresponding application protocol
	GetPortToProtocolMappingForService(service.MeshService) (map[uint32]string, error)

	// Returns the expected endpoints that are to be reached when the service FQDN is resolved under
	// the scope of the provider
	GetResolvableEndpointsForService(service.MeshService) ([]Endpoint, error)

	// GetID returns the unique identifier of the EndpointsProvider.
	GetID() string

	// GetAnnouncementsChannel obtains the channel on which providers will announce changes to the infrastructure.
	GetAnnouncementsChannel() <-chan announcements.Announcement
}

// Endpoint is a tuple of IP and Port representing an instance of a service
type Endpoint struct {
	IP      net.IP `json:"ip"`
	Port    Port   `json:"port"`
	PodName string `json:"podName"`
}

func (ep Endpoint) String() string {
	return fmt.Sprintf("(ip=%s, port=%d, podName=%s)", ep.IP, ep.Port, ep.PodName)
}

// Port is a numerical type representing a port on which a service is exposed
type Port uint32
