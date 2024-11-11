package certdb

import (
	"fmt"

	"github.com/redhat-best-practices-for-k8s/oct/pkg/certdb/offlinecheck"
	"github.com/redhat-best-practices-for-k8s/oct/pkg/certdb/onlinecheck"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
)

type CertificationStatusValidator interface {
	IsContainerCertified(registry, repository, tag, digest string) bool
	IsOperatorCertified(csvName, ocpVersion string) bool
	IsHelmChartCertified(helm *release.Release, ourKubeVersion string) bool
}

func GetValidator(offlineDBPath string) (CertificationStatusValidator, error) {
	// use the online certificator by default
	onlineValidator := onlinecheck.NewOnlineValidator()
	if onlineValidator.IsServiceReachable() {
		return onlineValidator, nil
	}

	// use the offline DB for disconnected environments
	logrus.Warnf("Online catalog not available. Testing with offline DB.")
	err := offlinecheck.LoadCatalogs(offlineDBPath)
	if err != nil {
		return nil, fmt.Errorf("offline DB not available, err: %v", err)
	}

	return offlinecheck.OfflineValidator{}, nil
}
