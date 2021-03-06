package lds

import (
	"fmt"

	xds_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	xds_listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	xds_tcp_proxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/wrapperspb"

	// "github.com/openservicemesh/osm/pkg/configurator"
	"github.com/openservicemesh/osm/pkg/envoy"
	"github.com/openservicemesh/osm/pkg/envoy/route"
	"github.com/openservicemesh/osm/pkg/service"
)

const (
	inboundMeshHTTPFilterChainName = "inbound-mesh-http-filter-chain"
	inboundMeshTCPFilterChain      = "inbound-mesh-tcp-filter-chain"
	httpAppProtocol                = "http"
	tcpAppProtocol                 = "tcp"
)

var (
	// supportedDownstreamHTTPProtocols is the list of allowed HTTP protocols that the
	// downstream can use in an HTTP request. Since the downstream client is only allowed
	// to send plaintext traffic to an in-mesh destinations, we do not include HTTP2 over
	// TLS (h2) in this list.
	supportedDownstreamHTTPProtocols = []string{"http/1.0", "http/1.1", "h2c"}
)

func (lb *listenerBuilder) getInboundMeshFilterChains(proxyService service.MeshService) []*xds_listener.FilterChain {
	var filterChains []*xds_listener.FilterChain

	protocolToPortMap, err := lb.meshCatalog.GetPortToProtocolMappingForService(proxyService)
	if err != nil {
		log.Error().Err(err).Msgf("Error retrieving port to protocol mapping for service %s", proxyService)
		return filterChains
	}

	// Create protocol specific inbound filter chains per port to handle different ports serving different protocols
	for port, appProtocol := range protocolToPortMap {
		switch appProtocol {
		case httpAppProtocol:
			// Filter chain for HTTP port
			filterChainForPort, err := lb.getInboundMeshHTTPFilterChain(proxyService, port)
			if err != nil {
				log.Error().Err(err).Msgf("Error building inbound HTTP filter chain for proxy:port %s:%d", proxyService, port)
				continue // continue building filter chains for other ports on the service
			}
			filterChains = append(filterChains, filterChainForPort)

		case tcpAppProtocol:
			filterChainForPort, err := lb.getInboundMeshTCPFilterChain(proxyService, port)
			if err != nil {
				log.Error().Err(err).Msgf("Error building inbound TCP filter chain for proxy:port %s:%d", proxyService, port)
				continue // continue building filter chains for other ports on the service
			}
			filterChains = append(filterChains, filterChainForPort)

		default:
			log.Error().Msgf("Cannot build inbound filter chain, unsupported protocol %s for proxy:port %s:%d", appProtocol, proxyService, port)
		}
	}

	return filterChains
}

func (lb *listenerBuilder) getInboundHTTPFilters(proxyService service.MeshService) ([]*xds_listener.Filter, error) {
	var filters []*xds_listener.Filter

	// Apply an RBAC filter when permissive mode is disabled. The RBAC filter must be the first filter in the list of filters.
	/* WITESAND_DISABLE
	 * we don't need RBAC
	if !lb.cfg.IsPermissiveTrafficPolicyMode() {
		// Apply RBAC policies on the inbound filters based on configured policies
		rbacFilter, err := lb.buildRBACFilter()
		if err != nil {
			log.Error().Err(err).Msgf("Error applying RBAC filter for proxy service %s", proxyService)
			return nil, err
		}
		// RBAC filter should be the very first filter in the filter chain
		filters = append(filters, rbacFilter)
	}
	*/

	// Apply the HTTP Connection Manager Filter
	inboundConnManager := getHTTPConnectionManager(route.InboundRouteConfigName, lb.cfg)
	marshalledInboundConnManager, err := ptypes.MarshalAny(inboundConnManager)
	if err != nil {
		log.Error().Err(err).Msgf("Error marshalling inbound HttpConnectionManager for proxy  service %s", proxyService)
		return nil, err
	}
	httpConnectionManagerFilter := &xds_listener.Filter{
		Name: wellknown.HTTPConnectionManager,
		ConfigType: &xds_listener.Filter_TypedConfig{
			TypedConfig: marshalledInboundConnManager,
		},
	}
	filters = append(filters, httpConnectionManagerFilter)

	return filters, nil
}

func (lb *listenerBuilder) getInboundMeshHTTPFilterChain(proxyService service.MeshService, servicePort uint32) (*xds_listener.FilterChain, error) {
	// Construct HTTP filters
	filters, err := lb.getInboundHTTPFilters(proxyService)
	if err != nil {
		log.Error().Err(err).Msgf("Error constructing inbound HTTP filters for proxy service %s", proxyService)
		return nil, err
	}

	// Construct downstream TLS context
	/* WITESAND_TLS_DISABLE
	marshalledDownstreamTLSContext, err := ptypes.MarshalAny(envoy.GetDownstreamTLSContext(proxyService, true))
	if err != nil {
		log.Error().Err(err).Msgf("Error marshalling DownstreamTLSContext for proxy service %s", proxyService)
		return nil, err
	}
	*/

	filterchainName := fmt.Sprintf("%s:%d", inboundMeshHTTPFilterChainName, servicePort)
	filterChain := &xds_listener.FilterChain{
		Name:    filterchainName,
		Filters: filters,

		// The 'FilterChainMatch' field defines the criteria for matching traffic against filters in this filter chain
		FilterChainMatch: &xds_listener.FilterChainMatch{
			// The DestinationPort is the service port the downstream directs traffic to
			DestinationPort: &wrapperspb.UInt32Value{
				Value: servicePort,
			},

			/* WITESAND_TLS_DISABLE
			// The ServerName is the SNI set by the downstream in the UptreamTlsContext by GetUpstreamTLSContext()
			// This is not a field obtained from the mTLS Certificate.
			// Only match when transport protocol is TLS
			ServerNames: []string{proxyService.ServerName()},

			TransportProtocol: envoy.TransportProtocolTLS,

			// In-mesh proxies will advertise this, set in the UpstreamTlsContext by GetUpstreamTLSContext()
			ApplicationProtocols: envoy.ALPNInMesh,
			*/
		},
		/* WITESAND_TLS_DISABLE
		TransportSocket: &xds_core.TransportSocket{
			Name: wellknown.TransportSocketTls,
			ConfigType: &xds_core.TransportSocket_TypedConfig{
				TypedConfig: marshalledDownstreamTLSContext,
			},
		},
		*/
	}

	return filterChain, nil
}

func (lb *listenerBuilder) getInboundMeshTCPFilterChain(proxyService service.MeshService, servicePort uint32) (*xds_listener.FilterChain, error) {
	// Construct TCP filters
	filters, err := lb.getInboundTCPFilters(proxyService)
	if err != nil {
		log.Error().Err(err).Msgf("Error constructing inbound TCP filters for proxy service %s", proxyService)
		return nil, err
	}

	// Construct downstream TLS context
	/* WITESAND_TLS_DISABLE
	marshalledDownstreamTLSContext, err := ptypes.MarshalAny(envoy.GetDownstreamTLSContext(proxyService, true))
	if err != nil {
		log.Error().Err(err).Msgf("Error marshalling DownstreamTLSContext for proxy service %s", proxyService)
		return nil, err
	}
	*/

	filterchainName := fmt.Sprintf("%s:%d", inboundMeshTCPFilterChain, servicePort)
	return &xds_listener.FilterChain{
		Name: filterchainName,
		FilterChainMatch: &xds_listener.FilterChainMatch{
			// The DestinationPort is the service port the downstream directs traffic to
			DestinationPort: &wrapperspb.UInt32Value{
				Value: servicePort,
			},

			// The ServerName is the SNI set by the downstream in the UptreamTlsContext by GetUpstreamTLSContext()
			// This is not a field obtained from the mTLS Certificate.
			ServerNames: []string{proxyService.ServerName()},

			// Only match when transport protocol is TLS
			/* WITESAND_TLS_DISABLE
			TransportProtocol: envoy.TransportProtocolTLS,

			// In-mesh proxies will advertise this, set in the UpstreamTlsContext by GetUpstreamTLSContext()
			ApplicationProtocols: envoy.ALPNInMesh,
			*/
		},
		Filters: filters,
		/* WITESAND_TLS_DISABLE
		TransportSocket: &xds_core.TransportSocket{
			Name: wellknown.TransportSocketTls,
			ConfigType: &xds_core.TransportSocket_TypedConfig{
				TypedConfig: marshalledDownstreamTLSContext,
			},
		},
		*/
	}, nil
}

func (lb *listenerBuilder) getInboundTCPFilters(proxyService service.MeshService) ([]*xds_listener.Filter, error) {
	var filters []*xds_listener.Filter

	// Apply an RBAC filter when permissive mode is disabled. The RBAC filter must be the first filter in the list of filters.
	if !lb.cfg.IsPermissiveTrafficPolicyMode() {
		// Apply RBAC policies on the inbound filters based on configured policies
		rbacFilter, err := lb.buildRBACFilter()
		if err != nil {
			log.Error().Err(err).Msgf("Error applying RBAC filter for proxy service %s", proxyService)
			return nil, err
		}
		// RBAC filter should be the very first filter in the filter chain
		filters = append(filters, rbacFilter)
	}

	// Apply the TCP Proxy Filter
	tcpProxy := &xds_tcp_proxy.TcpProxy{
		StatPrefix:       "inbound-mesh-tcp-proxy",
		ClusterSpecifier: &xds_tcp_proxy.TcpProxy_Cluster{Cluster: envoy.GetLocalClusterNameForService(proxyService)},
	}
	marshalledTCPProxy, err := ptypes.MarshalAny(tcpProxy)
	if err != nil {
		log.Error().Err(err).Msgf("Error marshalling TcpProxy object for egress HTTPS filter chain")
		return nil, err
	}
	tcpProxyFilter := &xds_listener.Filter{
		Name:       wellknown.TCPProxy,
		ConfigType: &xds_listener.Filter_TypedConfig{TypedConfig: marshalledTCPProxy},
	}
	filters = append(filters, tcpProxyFilter)

	return filters, nil
}

// getOutboundHTTPFilter returns an HTTP connection manager network filter used to filter outbound HTTP traffic
func (lb *listenerBuilder) getOutboundHTTPFilter() (*xds_listener.Filter, error) {
	var marshalledFilter *any.Any
	var err error

	marshalledFilter, err = ptypes.MarshalAny(
		getHTTPConnectionManager(route.OutboundRouteConfigName, lb.cfg))
	if err != nil {
		log.Error().Err(err).Msgf("Error marshalling HTTP connection manager object")
		return nil, err
	}

	return &xds_listener.Filter{
		Name:       wellknown.HTTPConnectionManager,
		ConfigType: &xds_listener.Filter_TypedConfig{TypedConfig: marshalledFilter},
	}, nil
}

// getOutboundHTTPFilterChainMatchForService builds a filter chain to match the HTTP baseddestination traffic.
// Filter Chain currently matches on the following:
// 1. Destination IP of service endpoints
// 2. HTTP application protocols
func (lb *listenerBuilder) getOutboundHTTPFilterChainMatchForService(dstSvc service.MeshService) (*xds_listener.FilterChainMatch, error) {
	filterMatch := &xds_listener.FilterChainMatch{
		// HTTP filter chain should only match on supported HTTP protocols that the downstream can use
		// to originate a request.
		ApplicationProtocols: supportedDownstreamHTTPProtocols,
	}

	endpoints, err := lb.meshCatalog.GetResolvableServiceEndpoints(dstSvc)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting GetResolvableServiceEndpoints for %q", dstSvc)
		return nil, err
	}

	if len(endpoints) == 0 {
		err := errors.Errorf("Endpoints not found for service %q", dstSvc)
		log.Error().Err(err).Msgf("Error constructing HTTP filter chain match for service %q", dstSvc)
		return nil, err
	}

	for _, endp := range endpoints {
		filterMatch.PrefixRanges = append(filterMatch.PrefixRanges, &xds_core.CidrRange{
			AddressPrefix: endp.IP.String(),
			PrefixLen: &wrapperspb.UInt32Value{
				Value: singleIpv4Mask,
			},
		})
	}

	return filterMatch, nil
}

func (lb *listenerBuilder) getOutboundHTTPFilterChainForService(upstream service.MeshService) (*xds_listener.FilterChain, error) {
	// Get HTTP filter for service
	filter, err := lb.getOutboundHTTPFilter()
	if err != nil {
		log.Error().Err(err).Msgf("Error getting HTTP filter for upstream service %s", upstream)
		return nil, err
	}

	// Get filter match criteria for destination service
	filterChainMatch, err := lb.getOutboundHTTPFilterChainMatchForService(upstream)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting HTTP filter chain match for upstream service %s", upstream)
		return nil, err
	}

	return &xds_listener.FilterChain{
		Name:             upstream.String(),
		Filters:          []*xds_listener.Filter{filter},
		FilterChainMatch: filterChainMatch,
	}, nil
}

/* WITESAND specific */
func (lb *listenerBuilder) getInboundInMeshFilterChain(proxyServiceName service.MeshService) (*xds_listener.FilterChain, error) {
	/* WITESAND_TLS_DISABLE
	marshalledDownstreamTLSContext, err := envoy.MessageToAny(envoy.GetDownstreamTLSContext(proxyServiceName, true ))
	if err != nil {
		log.Error().Err(err).Msgf("Error marshalling DownstreamTLSContext object for proxy %s", proxyServiceName)
		return nil, err
	}
	*/

	inboundConnManager := getHTTPConnectionManager(route.InboundRouteConfigName, lb.cfg)
	marshalledInboundConnManager, err := ptypes.MarshalAny(inboundConnManager)
	if err != nil {
		log.Error().Err(err).Msgf("Error marshalling inbound HttpConnectionManager object for proxy %s", proxyServiceName)
		return nil, err
	}

	filterChain := &xds_listener.FilterChain{
		Filters: []*xds_listener.Filter{
			{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &xds_listener.Filter_TypedConfig{
					TypedConfig: marshalledInboundConnManager,
				},
			},
		},

		// Apply this filter chain only to requests where the auth.UpstreamTlsContext.Sni matches
		// one from the list of ServerNames provided below.
		// This field is configured by the GetDownstreamTLSContext() function.
		// This is not a field obtained from the mTLS Certificate.

		/* WITESAND_TLS_DISABLE
		FilterChainMatch: &xds_listener.FilterChainMatch{
			ServerNames:          []string{proxyServiceName.GetCommonName().String()},
			TransportProtocol:    envoy.TransportProtocolTLS,
			ApplicationProtocols: envoy.ALPNInMesh, // in-mesh proxies will advertise this, set in UpstreamTlsContext
		},
		TransportSocket: &xds_core.TransportSocket{
			Name: wellknown.TransportSocketTls,
			ConfigType: &xds_core.TransportSocket_TypedConfig{
				TypedConfig: marshalledDownstreamTLSContext,
			},
		},
		*/
	}

	return filterChain, nil
}

