/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Note: the example only works with the code within the same release/branch.
package k8s

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"k8s.io/client-go/rest"
	"net/http"
	"crypto/tls"
	"os"
	"io"
	"encoding/json"
)

type MetricsFetcher struct {
	clientset *kubernetes.Clientset
	config *rest.Config
	httpClient *http.Client
}

type PrometheusResponse struct {
	Status string `json:"status"`
	Data struct {
		ResultType string `json:"resultType"`
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type MetricsConfig struct {
	PrometheusNamespace string
	PrometheusService string
	PrometheusPort int
	Query string
	Timeout time.Duration
}

type Options struct {
	KubeConfig string
	HTTPTimeout time.Duration
	InsecureSkipVerify bool
}

func DefaultOptions() *Options {
	return &Options {
		HTTPTimeout: 30 * time.Second,
		InsecureSkipVerify: false,
	}
}

// Original simple function
func GetKubeNode() (string, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {
		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		// Examples for error handling:
		// - Use helper functions like e.g. errors.IsNotFound()
		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
		namespace := "default"
		pod := "example-xxxxx"
		_, err = clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod %s in namespace %s not found\n", pod, namespace)
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %s in namespace %s: %v\n",
				pod, namespace, statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found pod %s in namespace %s\n", pod, namespace)
		}

		time.Sleep(10 * time.Second)
	}

	return "Kuberntes Call Finished", nil
}

func NewMetricsFetcher(opts *Options) (*MetricsFetcher, error){
	if opts == nil {
		opts = DefaultOptions()
	}

	config, err := getKubeConfig(opts.KubeConfig)

	if err != nil {
		return nil, fmt.Errorf("Failed to fetch kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create clientset: %w", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.InsecureSkipVerify,
			},
		},
		Timeout: opts.HTTPTimeout,
	}

	return &MetricsFetcher{
		clientset: clientset,
		config: config,
		httpClient: httpClient,
	}, nil
}

func getKubeConfig(kubeconfigPath string) (*rest.Config, error){
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			if home := homedir.HomeDir(); home != "" {
				kubeconfigPath = filepath.Join(home, ".kube", "config")
			}
		}
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// FetcMetrics via prometheus service directly

func (mf *MetricsFetcher) FetchMetricsViaService(ctx context.Context, config MetricsConfig) (*PrometheusResponse, error) {
	// Get prometheus service
	service, err := mf.clientset.CoreV1().Services(config.PrometheusNamespace).Get(
		ctx,
		config.PrometheusService,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to get prometheus service", err)
	}

	// Construct the url
	prometheusURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/api/v1/query",
		service.Name,
		service.Namespace,
		config.PrometheusPort,
	)

	return mf.queryPrometheus(ctx, prometheusURL, config.Query)
}

// FetchMetricsViaProxy fetch via kubernetes proxy
func (mf *MetricsFetcher) FetchMetricsViaProxy(ctx context.Context, config MetricsConfig) (*PrometheusResponse, error){
	// Construct proxy URL
	proxyPath := fmt.Sprintf("/api/v1/namespaces/%s/services/%s:%d/proxy/api/v1/query",
		config.PrometheusNamespace,
		config.PrometheusService,
		config.PrometheusPort,
	)

	req := mf.clientset.CoreV1().RESTClient().
		Get().
		AbsPath(proxyPath).
		Param("query", config.Query).
		Timeout(config.Timeout)

	result := req.Do(ctx)
	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("proxy request failed: %w", err)
	}

	body, err := result.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response PrometheusResponse 
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (mf *MetricsFetcher) queryPrometheus(ctx context.Context, baseURL, query string) (*PrometheusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := mf.httpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("Request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	var response PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// GetClientset 
func (mf *MetricsFetcher) GetClientSet() kubernetes.Interface {
	return mf.clientset
}

// Close cleans up
func (mf *MetricsFetcher) Close() error {
	return nil
}

// QueryNodeCPU
func (mf *MetricsFetcher) QueryNodeCPU(ctx context.Context, namespace, service string) (*PrometheusResponse, error) {
	config := MetricsConfig{
		PrometheusNamespace: namespace,
		PrometheusService: service,
		PrometheusPort: 9090,
		Query: `100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])))`,
		Timeout: 10 * time.Second,
	}

	return mf.FetchMetricsViaProxy(ctx, config)
}

// QueryNodeMemory
func (mf *MetricsFetcher) QueryNodeMemory(ctx context.Context, namespace, service string) (*PrometheusResponse, error) {
	config := MetricsConfig{
		PrometheusNamespace: namespace,
		PrometheusService: service,
		PrometheusPort: 9090,
		Query: `(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100`,
		Timeout: 10 * time.Second,
	}

	return mf.FetchMetricsViaProxy(ctx, config)
}

// QueryPodCPU
func (mf *MetricsFetcher) QueryPodCPU(ctx context.Context, promNamespace, promService, targetNamespace string) (*PrometheusResponse, error) {
	config := MetricsConfig{
		PrometheusNamespace: promNamespace,
		PrometheusService: promService,
		PrometheusPort: 9090,
		Query: fmt.Sprintf(`sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="%s"}[5m])) *100`, targetNamespace),
		Timeout: 10 * time.Second,
	}
	return mf.FetchMetricsViaProxy(ctx, config)
}

// QueryPodMemory
func (mf *MetricsFetcher) QueryPodMemory(ctx context.Context, promNamespace, promService, targetNamespace string) (*PrometheusResponse, error) {
	config := MetricsConfig{
		PrometheusNamespace: promNamespace,
		PrometheusService: promService,
		PrometheusPort: 9090,
		Query: fmt.Sprintf(`sum by (pod) (rate(container_memory_usage_bytes{namespace="%s"})) *100`, targetNamespace),
		Timeout: 10 * time.Second,
	}
	return mf.FetchMetricsViaProxy(ctx, config)
}


// QueryCustom allow custom PromQL queries
func (mf *MetricsFetcher) QueryCustom(ctx context.Context, config MetricsConfig) (*PrometheusResponse, error){
	return mf.FetchMetricsViaProxy(ctx, config)
}
