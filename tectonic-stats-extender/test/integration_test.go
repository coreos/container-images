package test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// These consts are extension name's we want to ensure are present in
	// BigQuery results.
	accountIDExtension              = "accountID"
	certificatesStrategyExtension   = "certificatesStrategy"
	installerPlatformExtension      = "installerPlatform"
	tectonicUpdaterEnabledExtension = "tectonicUpdaterEnabled"
	// extensionsNameKey is the key for extension names in BigQuery results.
	extensionsNameKey = "extensions_name"
	// extensionsValueKey is the key for extension values in BigQuery results.
	extensionsValueKey = "extensions_value"
	// tectonicSystemNamespace is the namespace for Tectonic.
	tectonicSystemNamespace = "tectonic-system"
)

var (
	// timeout is the maximum time for a test.
	timeout time.Duration
	// bigQuerySpec is the spec of the BigQuery table to test for cluster metrics.
	bigQuerySpec string
)

func TestMain(m *testing.M) {
	flag.DurationVar(&timeout, "timeout", 1*time.Minute, "maximum time for a test (default 1m)")
	flag.StringVar(&bigQuerySpec, "bigqueryspec", "", "BigQuery Spec (formatted as `bigquery://project.dataset.table`)")
	flag.Parse()

	os.Exit(m.Run())
}

// Test is the only test suite run by default.
func Test(t *testing.T) {
	t.Run("StatsEmitterLogs", testGetStatsEmitterLogs)
	t.Run("BigQueryData", testGetBigQueryData)
}

// newClient will attempt to produce a client from a config file
// specified in the KUBECONFIG environment variable. If this environment
// variable is empty, then `BuildConfigFromFlags` automatically tries to
// get a config from the in-cluster configuration.
func newClient() (*kubernetes.Clientset, error) {
	path := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes cluster config: %v", err)
	}
	return kubernetes.NewForConfig(config)
}

func testPointToCondition(f func(*testing.T) error, t *testing.T) wait.ConditionFunc {
	return func() (bool, error) {
		if err := f(t); err != nil {
			t.Logf("failed with error: %v", err)
			t.Log("retrying...")
			return false, nil
		}
		return true, nil
	}
}

func getStatsEmitterLogs(t *testing.T) error {
	c, err := newClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %v", err)
	}

	expected := "report successfully sent"
	namespace := "tectonic-system"
	podPrefix := "tectonic-stats-emitter"
	logs, err := validatePodLogging(c, namespace, podPrefix)
	if err != nil {
		return fmt.Errorf("failed to gather logs for %s/%s, %v", namespace, podPrefix, err)
	}
	if !bytes.Contains(logs, []byte(expected)) {
		return fmt.Errorf("expected logs to contain %q", expected)
	}
	return nil
}

func testGetStatsEmitterLogs(t *testing.T) {
	err := wait.Poll(5*time.Second, timeout, testPointToCondition(getStatsEmitterLogs, t))
	if err != nil {
		t.Fatalf("Failed to verify stats-emitter success in logs in %v.", timeout)
	}
	t.Log("Successfully verified stats-emitter success in logs.")
}

// getBigQueryData finds the Tectonic cluster ID from the Tectonic configmap
// in Kubernetes and uses it, along with the provided BigQuery spec, to query
// BigQuery for the metrics for the given cluster.
func getBigQueryData(t *testing.T) error {
	// Parse BigQuery spec.
	project, dataset, table, err := parseBigQuerySpec(bigQuerySpec)
	if err != nil {
		return fmt.Errorf("failed to parse BigQuery spec: %v", err)
	}
	// Get Tectonic cluster configuration.
	cm, err := getTectonicClusterConfig(t)
	if err != nil {
		return fmt.Errorf("failed to get Tectonic cluster configuration: %v", err)
	}
	cid, ok := cm.Data["clusterID"]
	if !ok {
		return errors.New("failed to find cluster ID in ConfigMap")
	}
	// Initialize BigQuery client.
	ctx := context.Background()
	// This assumes that:
	//  a) a GCE ServiceAccount has been created for this app
	//  b) the ServiceAccount is an owner of the dataset for this app
	//  c) the credentials for the ServiceAccount are in a file
	//  d) env GOOGLE_APPLICATION_CREDENTIALS=<path to credentials file>
	bq, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to create BigQuery client: %v", err)
	}
	// Get cluster stats extensions from BigQuery.
	q := bq.Query(`SELECT
 extensions.name,
 extensions.value,
FROM
  FLATTEN([` + fmt.Sprintf("%s:%s.%s", project, dataset, table) + `], extensions)
WHERE
 clusterID = '` + cid + `'
GROUP BY
 extensions.name,
 extensions.value`)
	expected := make(map[string]string)
	found := make(map[string]string)
	// extensions is an array of the tested stats extensions.
	var extensions = []string{accountIDExtension, certificatesStrategyExtension, installerPlatformExtension, tectonicUpdaterEnabledExtension}
	for _, name := range extensions {
		// Some extensions are not in the ConfigMap and so do not have
		// expected values. Instead, we just expect them to be present
		// in BigQuery and do not care about their values.
		if value, ok := cm.Data[name]; ok {
			expected[name] = value
		}
	}
	it, err := q.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to read query results: %v", err)
	}
	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to get next row: %v", err)
		}
		n, _ := row[extensionsNameKey]
		name, ok := n.(string)
		if !ok {
			return fmt.Errorf("expected extension name to be a string")
		}
		v, _ := row[extensionsValueKey]
		value, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected extension value to be a string")
		}
		found[name] = value
	}
	// Ensure stats extensions are in BigQuery.
	var wrong []string
	for _, name := range extensions {
		expectedValue, ok := expected[name]
		// If the extension does not have an expected value,
		// then just check if it is present in BigQuery at all.
		if !ok {
			if _, ok := found[name]; !ok {
				wrong = append(wrong, fmt.Sprintf("did not find extension %q", name))
			}
			continue
		}
		if foundValue, _ := found[name]; foundValue != expectedValue {
			wrong = append(wrong, fmt.Sprintf("expected extension %q to be %q, got %q", name, expectedValue, foundValue))
		}
	}
	if len(wrong) != 0 {
		return fmt.Errorf("failed to find extensions in BigQuery results: %s", strings.Join(wrong, "; "))
	}
	return nil
}

func testGetBigQueryData(t *testing.T) {
	if bigQuerySpec == "" {
		t.Skip("skipping because no BigQuery spec is defined")
		return
	}

	err := wait.Poll(10*time.Second, timeout, testPointToCondition(getBigQueryData, t))
	if err != nil {
		t.Fatalf("Failed to verify stats-emitter data in BigQuery in %v.", timeout)
	}
	t.Log("Successfully verified stats-emitter data in BigQuery.")
}

// bqre is a regular expression for parse BigQuery specs.
var bqre = regexp.MustCompile(`^bigquery://([^.]+)\.([^.]+)\.([^.]+)$`)

// parseBigQuerySpec parses a spec formatted as `bigquery://project.dataset.table`.
// The 3 string returns are project, dataset, and table respectively.
// This will return an error if it does not believe the argument is a BigQuery spec,
// or if it believes the argument is a biquery spec but it can't parse it properly.
func parseBigQuerySpec(spec string) (string, string, string, error) {
	if !strings.HasPrefix(spec, "bigquery://") {
		return "", "", "", errors.New("BigQuery spec must begin with \"bigquery://\"")
	}
	subs := bqre.FindStringSubmatch(spec)
	if len(subs) != 4 {
		return "", "", "", fmt.Errorf("invalid BigQuery spec: %q", spec)
	}
	return subs[1], subs[2], subs[3], nil
}

// getTectonicClusterConfig gets the cluster's configuration from the tectonic-config ConfigMap.
func getTectonicClusterConfig(t *testing.T) (*v1.ConfigMap, error) {
	c, err := newClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes client: %v", err)
	}

	configmapName := "tectonic-config"
	cm, err := c.Core().ConfigMaps(tectonicSystemNamespace).Get(configmapName, meta_v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to find ConfigMap %q: %v", configmapName, err)
	}
	return cm, nil
}

// validatePodLogging verifies that logs can be retrieved for a container in Pod.
func validatePodLogging(c *kubernetes.Clientset, namespace, podPrefix string) ([]byte, error) {
	var allLogs []byte
	pods, err := c.Core().Pods(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return allLogs, fmt.Errorf("could not list pods: %v", err)
	}

	var found bool
	for _, p := range pods.Items {
		if !strings.HasPrefix(p.Name, podPrefix) {
			continue
		}
		found = true

		if len(p.Spec.Containers) == 0 {
			return allLogs, fmt.Errorf("%s pod has no containers", p.Name)
		}

		opt := v1.PodLogOptions{
			Container: p.Spec.Containers[0].Name,
		}
		result := c.Core().Pods(namespace).GetLogs(p.Name, &opt).Do()
		if err := result.Error(); err != nil {
			return allLogs, fmt.Errorf("failed to get pod logs: %v", err)
		}

		var statusCode int
		result.StatusCode(&statusCode)
		if statusCode/100 != 2 {
			return allLogs, fmt.Errorf("expected 200 from log response, got %d", statusCode)
		}

		logs, err := result.Raw()
		if err != nil {
			return allLogs, fmt.Errorf("failed to read logs: %v", err)
		}

		allLogs = append(allLogs, logs...)
	}
	if !found {
		return allLogs, fmt.Errorf("failed to find pods with prefix %q in namespace %q", podPrefix, namespace)
	}
	return allLogs, nil
}
