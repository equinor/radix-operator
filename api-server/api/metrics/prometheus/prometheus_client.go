package prometheus

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/metrics"
	"github.com/equinor/radix-operator/api-server/api/utils/logs"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type QueryAPI interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prometheusV1.Option) (model.Value, prometheusV1.Warnings, error)
}

var ErrComponentsIsRequired = errors.New("components is required and must not be empty")

type Client struct {
	api QueryAPI
}

// NewPrometheusClient Constructor for a Prometheus Metrics Client
func NewPrometheusClient(prometheusUrl string) (metrics.Client, error) {
	logger := logs.NewRoundtripLogger(func(e *zerolog.Event) {
		e.Str("PrometheusClient", "prometheus")
	})

	apiClient, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl, RoundTripper: logger(prometheusApi.DefaultRoundTripper)})
	if err != nil {
		return nil, errors.New("failed to create the Prometheus API PrometheusClient")
	}
	api := prometheusV1.NewAPI(apiClient)

	return NewClient(api), nil
}

func NewClient(api QueryAPI) metrics.Client {
	return &Client{api: api}
}

// GetCpuRequests returns a list of all pods with their CPU requets. The envName can be empty to return all environments.
func (c *Client) GetCpuRequests(ctx context.Context, appName, envName string, compNames []string) ([]metrics.LabeledResults, error) {
	namespace := getNamespace(appName, envName)
	compSelector := getCompSelector(compNames)
	query := fmt.Sprintf(`max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="", namespace=~%q,resource="cpu"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component=~%q, label_radix_app=%q}`, namespace, compSelector, appName)
	return c.queryVector(ctx, appName, query)
}

// GetCpuAverage returns a list of all pods with their average CPU usage. The envName can be empty to return all environments.
func (c *Client) GetCpuAverage(ctx context.Context, appName, envName string, compNames []string, duration string) ([]metrics.LabeledResults, error) {
	namespace := getNamespace(appName, envName)
	compSelector := getCompSelector(compNames)
	query := fmt.Sprintf(`max by(namespace, container, pod) (avg_over_time(irate(container_cpu_usage_seconds_total{container!="", namespace=~%q}[1m]) [%s:])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component=~%q, label_radix_app=%q}`, namespace, duration, compSelector, appName)
	return c.queryVector(ctx, appName, query)
}

// GetMemoryRequests returns a list of all pods with their Memory requets. The envName can be empty to return all environments.
func (c *Client) GetMemoryRequests(ctx context.Context, appName, envName string, compNames []string) ([]metrics.LabeledResults, error) {
	namespace := getNamespace(appName, envName)
	compSelector := getCompSelector(compNames)
	query := fmt.Sprintf(`max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="", namespace=~%q,resource="memory"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component=~%q, label_radix_app=%q}`, namespace, compSelector, appName)
	return c.queryVector(ctx, appName, query)
}

// GetMemoryMaximum returns a list of all pods with their maximum Memory usage. The envName can be empty to return all environments.
func (c *Client) GetMemoryMaximum(ctx context.Context, appName, envName string, compNames []string, duration string) ([]metrics.LabeledResults, error) {
	namespace := getNamespace(appName, envName)
	compSelector := getCompSelector(compNames)
	query := fmt.Sprintf(`max by(namespace, container, pod) (max_over_time(container_memory_working_set_bytes{container!="", namespace=~%q} [%s:])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component=~%q, label_radix_app=%q}`, namespace, duration, compSelector, appName)
	return c.queryVector(ctx, appName, query)
}

func getNamespace(appName, envName string) string {
	appName = regexp.QuoteMeta(appName)
	envName = regexp.QuoteMeta(envName)

	if envName == "" {
		return appName + "-.*"
	}

	return appName + "-" + envName
}

func getCompSelector(compNames []string) string {
	names := slice.Map(compNames, func(compName string) string { return regexp.QuoteMeta(compName) })
	return strings.Join(names, "|")
}

func (c *Client) queryVector(ctx context.Context, appName, query string) ([]metrics.LabeledResults, error) {
	response, w, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("query", query).Msg("fetching vector query")
		return nil, err
	}
	if len(w) > 0 {
		log.Ctx(ctx).Warn().Str("query", query).Strs("warnings", w).Msgf("fetching vector query")
	} else {
		log.Ctx(ctx).Trace().Str("query", query).Msgf("fetching vector query")
	}

	r, ok := response.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("queryVector returned non-vector response")
	}

	var result []metrics.LabeledResults
	for _, sample := range r {
		namespace := string(sample.Metric["namespace"])
		envName, _ := strings.CutPrefix(namespace, appName+"-")

		result = append(result, metrics.LabeledResults{
			Value:       float64(sample.Value),
			Environment: envName,
			Component:   string(sample.Metric["label_radix_component"]),
			Pod:         string(sample.Metric["pod"]),
		})
	}
	return result, nil
}
