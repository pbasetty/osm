package envoy

import (
	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/open-service-mesh/osm/pkg/certificate"
	"github.com/open-service-mesh/osm/pkg/endpoint"
)

const (
	svc = "service-name"
	ns  = "some-namespace"
)

var _ = Describe("Test proxy methods", func() {
	Context("Testing proxy.GetCommonName()", func() {
		It("should return DNS-1123 CN of the proxy", func() {
			commonNameForProxy := fmt.Sprintf("UUID-of-proxy.%s.%s.one.two.three.co.uk", svc, ns)
			commonNameForService := fmt.Sprintf("%s.%s.svc.cluster.local", svc, ns)
			cn := certificate.CommonName(commonNameForProxy)

			namespacedSvc := endpoint.NamespacedService{
				Namespace: ns,
				Service:   svc,
			}

			proxy := NewProxy(cn, namespacedSvc, net.IP{})

			actualCN := proxy.GetCommonName()
			Expect(actualCN).To(Equal(certificate.CommonName(commonNameForProxy)))
			actualServiceCN := proxy.ServiceName.GetCommonName()
			expectedServiceCN := certificate.CommonName(commonNameForService)
			Expect(actualServiceCN).To(Equal(expectedServiceCN))
		})
	})
})