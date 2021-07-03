package request

import (
	"bytes"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/andig/evcc/server/metrics"
	"github.com/andig/evcc/util"
	"github.com/prometheus/client_golang/prometheus"
)

var _ http.RoundTripper = (*RoundTripper)(nil)

type RoundTripper struct {
	log         *util.Logger
	base        http.RoundTripper
	MetricsPush bool
}

const max = 2048 * 2

var (
	labels                   = []string{"host"}
	reqMetric, reqMetricPush *prometheus.SummaryVec
	resMetric, resMetricPush *prometheus.CounterVec
)

func sumVec() *prometheus.SummaryVec {
	return prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "evcc",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "A summary of HTTP request durations",
		Objectives: map[float64]float64{
			0.5:  0.05,  // 50th percentile with a max. absolute error of 0.05
			0.9:  0.01,  // 90th percentile with a max. absolute error of 0.01
			0.99: 0.001, // 99th percentile with a max. absolute error of 0.001
		},
	}, labels)
}

func countVec() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "evcc",
		Subsystem: "http",
		Name:      "request_total",
		Help:      "Total count of HTTP requests",
	}, append(labels, "status"))
}

func init() {
	reqMetric = sumVec()
	reqMetricPush = sumVec()

	resMetric = countVec()
	resMetricPush = countVec()

	prometheus.MustRegister(reqMetric, resMetric)
	metrics.PushRegistry.MustRegister(reqMetricPush, resMetricPush)
}

// NewTripper creates a logging roundtrip handler
func NewTripper(log *util.Logger, base http.RoundTripper) *RoundTripper {
	tripper := &RoundTripper{
		log:  log,
		base: base,
	}

	return tripper
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (r *RoundTripper) updateReq(host string, d time.Duration) {
	reqMetric.WithLabelValues(host).Observe(d.Seconds())
	if r.MetricsPush {
		reqMetricPush.WithLabelValues(host).Observe(d.Seconds())
	}
}

func (r *RoundTripper) updateRes(host, status string) {
	resMetric.WithLabelValues(host, status).Add(1)
	if r.MetricsPush {
		resMetricPush.WithLabelValues(host, status).Add(1)
	}
}

func (r *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.log.TRACE.Printf("%s %s", req.Method, req.URL.String())

	var bld strings.Builder
	if body, err := httputil.DumpRequestOut(req, true); err == nil {
		bld.WriteString("\n")
		bld.Write(bytes.TrimSpace(body[:min(max, len(body))]))
	}

	startTime := time.Now()
	resp, err := r.base.RoundTrip(req)

	r.updateReq(req.URL.Hostname(), time.Since(startTime))

	if err == nil {
		r.updateRes(req.URL.Hostname(), strconv.Itoa(resp.StatusCode))

		if body, err := httputil.DumpResponse(resp, true); err == nil {
			bld.WriteString("\n\n")
			bld.Write(bytes.TrimSpace(body[:min(max, len(body))]))
		}
	} else {
		r.updateRes(req.URL.Hostname(), "999")
	}

	if bld.Len() > 0 {
		r.log.TRACE.Println(bld.String())
	}

	return resp, err
}
