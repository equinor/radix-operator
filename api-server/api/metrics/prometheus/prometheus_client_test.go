package prometheus_test

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/api-server/api/metrics/prometheus"
	mock2 "github.com/equinor/radix-operator/api-server/api/metrics/prometheus/mock"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestArguemtsExistsInQuery(t *testing.T) {

	ctrl := gomock.NewController(t)
	mock := mock2.NewMockQueryAPI(ctrl)

	gomock.InOrder(
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "app0-.*")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "app1")
				assert.Contains(t, query, "dev1")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "app2")
				assert.Contains(t, query, "dev2")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "app3")
				assert.Contains(t, query, "dev3")
				assert.Contains(t, query, "24h")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "app4")
				assert.Contains(t, query, "dev4")
				assert.Contains(t, query, "36h")

				return nil, nil, nil
			},
		),
	)

	client := prometheus.NewClient(mock)
	_, _ = client.GetCpuRequests(context.Background(), "app0", "", nil)
	_, _ = client.GetCpuRequests(context.Background(), "app1", "dev1", nil)
	_, _ = client.GetMemoryRequests(context.Background(), "app2", "dev2", nil)
	_, _ = client.GetCpuAverage(context.Background(), "app3", "dev3", nil, "24h")
	_, _ = client.GetMemoryMaximum(context.Background(), "app4", "dev4", nil, "36h")
}
