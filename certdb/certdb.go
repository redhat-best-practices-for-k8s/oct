package certdb

import (
	"fmt"

	"github.com/test-network-function/oct/certdb/offlinecheck"
	"helm.sh/helm/v3/pkg/release"
)

type CertificationStatusValidator interface {
	IsContainerCertified(registry, repository, tag, digest string) bool
	IsOperatorCertified(csvName, ocpVersion, channel string) bool
	IsHelmChartCertified(helm *release.Release, ourKubeVersion string) bool
}

func GetValidator(offlineDBPath string) (CertificationStatusValidator, error) {
	err := offlinecheck.LoadCatalogs(offlineDBPath)
	if err != nil {
		return nil, fmt.Errorf("offline DB not available, err: %v", err)
	}

	return offlinecheck.OfflineValidator{}, nil
}
