package yandex

import (
	"fmt"
	"regexp"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	regExpProviderID = regexp.MustCompile(`^` + providerName + `:///([^/]+)/([^/]+)$`)
)

// extractNodeAddresses maps the instance information to an array of NodeAddresses:
func extractNodeAddresses(instance *compute.Instance) ([]v1.NodeAddress, error) {
	nodeAddresses := []v1.NodeAddress{{Type: v1.NodeInternalDNS, Address: instance.Fqdn}}

	if instance.NetworkInterfaces != nil {
		for _, networkInterface := range instance.NetworkInterfaces {
			if networkInterface.PrimaryV4Address != nil {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: networkInterface.PrimaryV4Address.Address})
			}

			if networkInterface.PrimaryV4Address.OneToOneNat != nil {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: networkInterface.PrimaryV4Address.OneToOneNat.Address})
			}
		}
	}

	return nodeAddresses, nil
}

// mapNodeNameToInstanceName maps a k8s Node Name to a Yandex.Cloud Instance Name
// Currently - this is a simple string cast.
func mapNodeNameToInstanceName(nodeName types.NodeName) string {
	return string(nodeName)
}

// parseProviderID splits a providerID into Folder ID, Zone and Instance Name.
func parseProviderID(providerID string) (zone string, instanceName string, err error) {
	// providerID is in the following form "${providerName}:///${zone}/${instanceName}"
	// So for input "yandex:///ru-central1-a/e2e-test-node0" output will be  "ru-central1-a", "e2e-test-node0".
	matches := regExpProviderID.FindStringSubmatch(providerID)
	if len(matches) != 3 {
		return "", "", fmt.Errorf("unexpected input: %s", providerID)
	}

	return matches[1], matches[2], nil
}
