package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/protocol"
)

// PromHandler 输出prometheus指标。
type PromHandler struct {
	xl           *xlog.Logger
	pushJobName  string
	pushURL      string
	pushInterval time.Duration

	promHTTPHandler http.Handler
	pusher          *push.Pusher
}

const (
	// DefaultMetricsPath 默认的metrics HTTP服务的路径。
	DefaultMetricsPath = "/metrics"
	// DefaultPushJob 开启pushgateway时，默认的job名称。
	DefaultPushJob = "qlive-api"
	// DefaultPushInterval 开启pushgateway时，默认的推送时间间隔。
	DefaultPushInterval = 30 * time.Second
	// NanoSecondsInSecond 1秒中的纳秒数。
	NanoSecondsInSecond = 1000 * 1000 * 1000
)

var requestCountCollector = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "qlive",
	Subsystem: "api",
	Name:      "http_request_count",
	Help:      "number of HTTP requests by path, mehtod and status code",
}, []string{"method", "path", "code"})

var requestDurationCollector = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "qlive",
	Subsystem: "api",
	Name:      "http_request_duration",
	Help:      "duration of HTTP requests in seconds",
}, []string{"method", "path", "code"})

// NewPromHandler 创建prometheus handler.
func NewPromHandler(conf *config.PrometheusConfig, xl *xlog.Logger) *PromHandler {
	if xl == nil {
		xl = xlog.New("qlive-prometheus-handler")
	}

	h := &PromHandler{
		xl:              xl,
		promHTTPHandler: promhttp.Handler(),
	}
	prometheus.MustRegister(requestCountCollector, requestDurationCollector)
	if conf.EnablePush {
		var pushInterval time.Duration
		if conf.PushIntervalSeconds <= 0 {
			pushInterval = DefaultPushInterval
		} else {
			pushInterval = time.Duration(conf.PushIntervalSeconds) * time.Second
		}
		h.pushInterval = pushInterval

		if conf.PushJob == "" {
			h.pushJobName = DefaultPushJob
		}
		h.pushURL = conf.PushURL
		h.pusher = push.New(h.pushURL, h.pushJobName).Gatherer(prometheus.DefaultGatherer)
		go h.pushTick()
	}
	return h
}

func (h *PromHandler) pushTick() {

	ticker := time.NewTicker(h.pushInterval)
	for range ticker.C {
		err := h.pusher.Push()
		if err != nil {
			h.xl.Infof("push prometheus metrics error: %v", err)
		}
	}
}

// SetMetrics 设置prometheus 指标。
func SetMetrics(c *gin.Context) {
	startTime, hasStartTime := c.Get(protocol.RequestStartKey)
	var elapsed time.Duration
	if hasStartTime {
		elapsed = time.Now().Sub(startTime.(time.Time))
	}

	path := c.Request.URL.Path
	method := c.Request.Method
	statusCode := c.Writer.Status()

	// replace roomID in path to :roomID.
	for _, p := range c.Params {
		if p.Key == "roomID" {
			path = strings.Replace(path, p.Value, ":roomID", 1)
		}
	}

	requestCountCollector.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
	if hasStartTime {
		elapsedSeconds := float64(elapsed) / NanoSecondsInSecond
		requestDurationCollector.WithLabelValues(method, path, strconv.Itoa(statusCode)).Observe(elapsedSeconds)
	}
}

// HandleMetrics 返回prometeus metrics.
func (h *PromHandler) HandleMetrics(c *gin.Context) {
	h.promHTTPHandler.ServeHTTP(c.Writer, c.Request)
}
