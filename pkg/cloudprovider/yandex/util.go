package yandex

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

var (
	deprecatedRegExpProviderID = regexp.MustCompile(`^` + providerName + `://([^/]+)/([^/]+)/([^/]+)$`)
	regExpProviderID           = regexp.MustCompile(`^` + providerName + `://(.+)$`)
)

// GetRegion returns region of the provided zone.
func GetRegion(zoneName string) (string, error) {
	// zoneName is in the following form: ${regionName}-${ix}.
	// So for input "ru-central1-a" output will be "ru-central1".
	ix := strings.LastIndex(zoneName, "-")
	if ix == -1 {
		return "", fmt.Errorf("unexpected input: %s", zoneName)
	}

	return zoneName[:ix], nil
}

func MapNodeNameToInstanceName(nodeName types.NodeName) string {
	return string(nodeName)
}

func ParseProviderID(providerID string) (instanceName string, instanceNameIsId bool, err error) {
	deprecatedMatches := deprecatedRegExpProviderID.FindStringSubmatch(providerID)
	if len(deprecatedMatches) == 4 {
		return deprecatedMatches[3], false, nil
	}

	matches := regExpProviderID.FindStringSubmatch(providerID)
	if len(matches) == 1 {
		return matches[1], true, nil
	}

	return "", false, fmt.Errorf("can't parse providerID %q", providerID)
}
