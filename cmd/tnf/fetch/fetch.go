package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/redhat-best-practices-for-k8s/oct/pkg/certdb/offlinecheck"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	httpClientTimeout = 2 * time.Minute

	containersGraphQLIncludeFilter = "data._id," +
		"data.creation_date," +
		"data.repositories.registry," +
		"data.repositories.repository," +
		"data.repositories.manifest_list_digest," +
		"data.repositories.tags.name," +
		"data.image_id," +
		"data.architecture"

	operatorsGraphQLIncludeFilter = "page," +
		"page_size," +
		"total," +
		"data.csv_name," +
		"data.ocp_version," +
		"data.channel_name"
)

var (
	containersCatalogURL    = "https://catalog.redhat.com/api/containers/v1/images"
	operatorsCatalogSizeURL = "https://catalog.redhat.com/api/containers/v1/operators/bundles?filter=organization==certified-operators"
	// Pyxis include lists only fields consumed when loading operator pages (offlinecheck.OperatorCatalog / OperatorData).
	operatorsCatalogPageURL = "https://catalog.redhat.com/api/containers/v1/operators/bundles?filter=organization==certified-operators&page_size=%d&page=%d&include=" + operatorsGraphQLIncludeFilter
	helmCatalogURL          = "https://charts.openshift.io/index.yaml"
	containersRelativePath  = "%s/cmd/tnf/fetch/data/containers/containers.db"
	operatorsRelativePath   = "%s/cmd/tnf/fetch/data/operators/"
	helmRelativePath        = "%s/cmd/tnf/fetch/data/helm/helm.db"
	certifiedcatalogdata    = "%s/cmd/tnf/fetch/data/archive.json"
	operatorFileFormat      = "operator_catalog_page_%d_%d.db"
)

var (
	command = &cobra.Command{
		Use:   "fetch",
		Short: "fetch the list of certified operators and containers.",
		RunE:  RunCommand,
	}
	operatorFlag  = "operator"
	containerFlag = "container"
	helmFlag      = "helm"
	sinceFlag     = "since"
)

type CertifiedCatalog struct {
	Containers int `json:"containers"`
	Operators  int `json:"operators"`
	Charts     int `json:"charts"`
}

func NewCommand() *cobra.Command {
	command.PersistentFlags().BoolP(operatorFlag, "o", false,
		"if specified, the operators DB will be updated")
	command.PersistentFlags().BoolP(containerFlag, "c", false,
		"if specified, the certified containers DB will be updated")
	command.PersistentFlags().BoolP(helmFlag, "m", false,
		"if specified, the helm file will be updated")
	command.PersistentFlags().StringP(sinceFlag, "s", "",
		"only fetch entries newer than this value (Go duration like 1h/30m or RFC3339 timestamp)")
	return command
}

func parseSinceFlag(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}

	if d, err := time.ParseDuration(raw); err == nil {
		return time.Now().UTC().Add(-d), nil
	}

	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since value %q: must be a Go duration (e.g. 1h, 30m) or RFC3339 timestamp", raw)
	}

	return t.UTC(), nil
}

// RunCommand execute the fetch subcommands
func RunCommand(cmd *cobra.Command, _ []string) error {
	data := getCertifiedCatalogOnDisk()
	log.Infof("Current offline artifacts: %+v", data)

	sinceRaw, err := cmd.PersistentFlags().GetString(sinceFlag)
	if err != nil {
		return fmt.Errorf("failed to get --since flag: %w", err)
	}

	sinceTime, err := parseSinceFlag(sinceRaw)
	if err != nil {
		return fmt.Errorf("failed to parse --since flag: %w", err)
	}

	hasSince := !sinceTime.IsZero()

	b, err := cmd.PersistentFlags().GetBool(operatorFlag)
	if err != nil {
		log.Error("Can't process the flag, ", operatorFlag)
		return err
	} else if b {
		if hasSince {
			log.Warning("--since is not yet implemented for operators, ignoring")
		}
		err = getOperatorCatalog(&data)
		if err != nil {
			log.Fatalf("fetching operators failed: %v", err)
		}
	}
	b, err = cmd.PersistentFlags().GetBool(containerFlag)
	if err != nil {
		return err
	} else if b {
		err = getContainerCatalog(&data, sinceTime)
		if err != nil {
			log.Fatalf("fetching containers failed: %v", err)
		}
	}
	b, err = cmd.PersistentFlags().GetBool(helmFlag)
	if err != nil {
		return err
	} else if b {
		if hasSince {
			log.Warning("--since is not yet implemented for helm charts, ignoring")
		}
		err = getHelmCatalog()
		if err != nil {
			log.Fatalf("fetching helm charts failed: %v", err)
		}
	}

	log.Info(data)
	serializeData(data)
	return nil
}

// getHTTPBody helper function to get binary data from URL
func getHTTPBody(requestURL string) ([]uint8, error) {
	//nolint:gosec
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("http request %s failed with error: %w", requestURL, err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body from %s: %w, body: %s", requestURL, err, string(body))
	}
	return body, nil
}

func getCertifiedCatalogOnDisk() CertifiedCatalog {
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	filePath := fmt.Sprintf(certifiedcatalogdata, path)
	if _, err = os.Stat(filePath); err != nil {
		return CertifiedCatalog{0, 0, 0}
	}
	f, err := os.Open(filePath)
	if err != nil {
		log.Error("can't process file", err, " trying to proceed")
		return CertifiedCatalog{0, 0, 0}
	}
	defer f.Close()
	bytes, err := io.ReadAll(f)
	if err != nil {
		log.Error("can't process file", err, " trying to proceed")
	}
	var data CertifiedCatalog
	if err = yaml.Unmarshal(bytes, &data); err != nil {
		log.Error("error when parsing the data", err)
	}
	return data
}

func serializeData(data CertifiedCatalog) {
	start := time.Now()
	path, err := os.Getwd()
	if err != nil {
		log.Error("can't get current working dir", err)
		return
	}
	filename := fmt.Sprintf(certifiedcatalogdata, path)
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal("Couldn't open file")
	}
	log.Trace("marshall container db into file=", f.Name())
	defer f.Close()
	bytes, _ := json.Marshal(data)
	_, err = f.Write(bytes)
	if err != nil {
		log.Error(err)
	}
	log.Info("serialization time", time.Since(start))
}

func getOperatorCatalogSize() (size, pagesize uint, err error) {
	log.Infof("Getting operators catalog size, url: %s", operatorsCatalogSizeURL)

	body, err := getHTTPBody(operatorsCatalogSizeURL)
	if err != nil {
		return 0, 0, err
	}

	var aCatalog offlinecheck.OperatorCatalog
	err = json.Unmarshal(body, &aCatalog)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshall response from %s: %w, body: %s",
			operatorsCatalogSizeURL, err, string(body))
	}

	return aCatalog.Total, aCatalog.PageSize, nil
}

func getOperatorCatalogPage(page, size uint) error {
	path, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	requestURL := fmt.Sprintf(operatorsCatalogPageURL, size, page)
	log.Infof("Getting operators catalog page %d, url: %s", page, requestURL)

	body, err := getHTTPBody(requestURL)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf(operatorsRelativePath+"/"+operatorFileFormat, path, page, size)
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer f.Close()
	_, err = f.Write(body)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filename, err)
	}
	return nil
}

func getOperatorCatalog(data *CertifiedCatalog) error {
	start := time.Now()
	total, pageSize, err := getOperatorCatalogSize()
	if err != nil {
		return fmt.Errorf("failed to get operators catalog size: %w", err)
	}

	log.Infof("Certified operators in the online catalog: %d, page size: %d", total, pageSize)
	// convert data.Operators to uint to compare with total

	if total == uint(data.Operators) {
		log.Info("No new certified operator found")
		return nil
	}

	err = removeOperatorsDB()
	if err != nil {
		return fmt.Errorf("failed to remove operators db: %w", err)
	}

	pages := total / pageSize
	remaining := total - pages*pageSize
	log.Infof("Downloading %d pages of size %d plus another page for the %d remaining entries.",
		pages, pageSize, remaining)

	for page := uint(0); page < pages; page++ {
		err = getOperatorCatalogPage(page, pageSize)
		if err != nil {
			return fmt.Errorf("failed to get operators page %d (total %d)", page, total)
		}
	}
	if remaining != 0 {
		err = getOperatorCatalogPage(pages, remaining)
		if err != nil {
			return fmt.Errorf("failed to get remaining operators page %d (total %d)", pages, total)
		}
	}

	data.Operators = int(total)

	log.Info("Time to process all the operators: ", time.Since(start))
	return nil
}

func fetchContainerPage(client *http.Client, cursor string, sinceTime time.Time) ([]offlinecheck.ContainerCatalogEntry, error) {
	filter := fmt.Sprintf("isv_pid!=null and creation_date<%s and certified==true", cursor)
	if !sinceTime.IsZero() {
		filter += fmt.Sprintf(" and creation_date>=%s", sinceTime.Format(time.RFC3339))
	}

	params := url.Values{}
	params.Set("filter", filter)
	params.Set("include", containersGraphQLIncludeFilter)
	params.Set("sort_by", "creation_date[desc]")

	reqURL := containersCatalogURL + "?" + params.Encode()
	log.Infof("Request URL: %s", reqURL)

	start := time.Now()
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respTime := time.Since(start)
	log.Infof("Time to fetch json body: %d seconds (%d milliseconds)", int(respTime.Seconds()), int(respTime.Milliseconds()))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result offlinecheck.ContainerPageCatalog
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

func getContainerCatalogPage(client *http.Client, cursor string, sinceTime time.Time) ([]offlinecheck.ContainerCatalogEntry, error) {
	retryDelays := []time.Duration{0, 30 * time.Second, 1 * time.Minute, 2 * time.Minute}

	var lastErr error
	for attempt, delay := range retryDelays {
		if delay > 0 {
			log.Warningf("Attempt %d failed: %v. Retrying in %s...", attempt, lastErr, delay)
			time.Sleep(delay)
		}

		entries, err := fetchContainerPage(client, cursor, sinceTime)
		if err != nil {
			lastErr = err
			continue
		}

		return entries, nil
	}

	return nil, fmt.Errorf("all %d retry attempts exhausted: %w", len(retryDelays), lastErr)
}

func serializeContainersDB(db map[string]*offlinecheck.ContainerCatalogEntry) error {
	start := time.Now()
	log.Info("start serializing container catalog")
	path, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	filename := fmt.Sprintf(containersRelativePath, path)
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed go create file %s: %w", filename, err)
	}

	log.Trace("marshall container db into file=", f.Name())
	defer f.Close()
	bytes, _ := json.Marshal(db)
	_, err = f.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to write into file %s: %w", filename, err)
	}

	log.Info("serialization time=", time.Since(start))
	return nil
}

func parseCreationDate(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}

	return time.Parse("2006-01-02T15:04:05.000000+00:00", raw)
}

func processContainerEntries(
	entries []offlinecheck.ContainerCatalogEntry,
	db map[string]*offlinecheck.ContainerCatalogEntry,
	sinceTime time.Time,
) (int, error) {
	newCount := 0
	for i := range entries {
		if !sinceTime.IsZero() {
			ct, err := parseCreationDate(entries[i].CreationDate)
			if err != nil {
				return newCount, fmt.Errorf("failed to parse creation_date %q: %w", entries[i].CreationDate, err)
			}

			if ct.Before(sinceTime) {
				log.Infof("Entry %s has creation_date %s before --since cutoff, stopping.", entries[i].ID, entries[i].CreationDate)
				return newCount, nil
			}
		}

		if _, exists := db[entries[i].DockerImageDigest]; !exists {
			if entries[i].DockerImageDigest == "" {
				log.Warningf("Entry %s has no docker image digest, skipping it. Entry: %+v", entries[i].ID, entries[i])
				continue
			}

			// The certified field was not included in the initial Pyxis response, so we need to set it to true
			// Not strictly necessary for offline checks, but let's set it to true for consistency.
			entries[i].Certified = true

			db[entries[i].DockerImageDigest] = &entries[i]
			newCount++
		}
	}

	return newCount, nil
}

func getContainerCatalog(data *CertifiedCatalog, sinceTime time.Time) error {
	start := time.Now()
	db := make(map[string]*offlinecheck.ContainerCatalogEntry)

	if err := removeContainersDB(); err != nil {
		return fmt.Errorf("failed to remove containers db: %w", err)
	}

	client := &http.Client{Timeout: httpClientTimeout}
	cursor := time.Now().UTC().Format(time.RFC3339)
	total := 0

	if !sinceTime.IsZero() {
		log.Infof("Downloading containers catalog (since %s) using cursor-based pagination.", sinceTime.Format(time.RFC3339))
	} else {
		log.Info("Downloading containers catalog using cursor-based pagination.")
	}

	// We'll query using the pyxis index based on isv_pid and creation_date fields (desc order).
	// Pagination works by using the creation_date of the oldest entry in each response as the
	// cursor for the next request, continuing until no more entries are returned or the date
	// falls before the --since cutoff (if provided).
	for page := 0; ; page++ {
		log.Infof("Getting containers catalog page %d (cursor: %s)", page, cursor)

		entries, err := getContainerCatalogPage(client, cursor, sinceTime)
		if err != nil {
			return fmt.Errorf("failed to get containers page %d: %w", page, err)
		}

		if len(entries) == 0 {
			break
		}

		newCount, err := processContainerEntries(entries, db, sinceTime)
		if err != nil {
			return err
		}

		total += newCount
		cursor = entries[len(entries)-1].CreationDate
		log.Infof("Container page %d: %d entries (%d new, total: %d)", page, len(entries), newCount, total)

		if newCount == 0 {
			break
		}
	}

	if err := serializeContainersDB(db); err != nil {
		return fmt.Errorf("failed to serialize containers db: %w", err)
	}

	data.Containers = total
	log.Infof("Certified containers in the online catalog: %d", total)
	log.Info("Time to process all the containers: ", time.Since(start))

	return nil
}

func getHelmCatalog() error {
	start := time.Now()
	err := removeHelmDB()
	if err != nil {
		return err
	}

	log.Infof("Getting helm charts catalog page, url: %s", helmCatalogURL)
	body, err := getHTTPBody(helmCatalogURL)
	if err != nil {
		return err
	}

	path, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	filename := fmt.Sprintf(helmRelativePath, path)
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	_, err = f.Write(body)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filename, err)
	}

	log.Info("Time to process all the charts: ", time.Since(start))
	return nil
}

func removeContainersDB() error {
	path, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	filename := fmt.Sprintf(containersRelativePath, path)
	err = os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove file %s: %w", filename, err)
	}

	return nil
}
func removeHelmDB() error {
	path, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	filename := fmt.Sprintf(helmRelativePath, path)
	err = os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove file %s: %w", filename, err)
	}

	return nil
}
func removeOperatorsDB() error {
	path, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	path = fmt.Sprintf(operatorsRelativePath, path)
	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read %s files: %w", path, err)
	}
	for _, file := range files {
		filePath := fmt.Sprintf("%s/%s", path, file.Name())
		if err = os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove file %s: %w", filePath, err)
		}
	}

	return nil
}
