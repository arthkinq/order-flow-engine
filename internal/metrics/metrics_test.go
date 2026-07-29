package metrics_test

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/metrics"
	"github.com/ozontech/testo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MetricsSuite struct {
	testo.Suite[*testo.T]
}

func (MetricsSuite) TestStartServer_ExposesMetricsEndpoint(t *testo.T) {
	addr := "127.0.0.1:2114"
	metrics.StartServer(addr)

	metrics.OrdersCreatedTotal.Inc()
	metrics.OrdersProcessedTotal.WithLabelValues("completed").Inc()
	metrics.OrderProcessingDuration.Observe(0.05)

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + addr + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	content := string(body)
	assert.Contains(t, content, "orders_created_total")
	assert.Contains(t, content, "orders_processed_total")
	assert.Contains(t, content, "order_processing_duration_seconds")
}

func TestMetricsSuite(t *testing.T) {
	testo.RunSuite(t, new(MetricsSuite))
}
