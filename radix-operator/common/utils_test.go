package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForValues(t *testing.T) {
	t.Parallel()
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancelFn()

	sync := make(chan int)
	go func() {
		time.Sleep(1 * time.Millisecond)
		sync <- 1
		time.Sleep(1 * time.Millisecond)
		sync <- 2
	}()
	vals, err := WaitForValues(ctx, sync, 2)
	assert.NoError(t, err)
	require.Len(t, vals, 2)
	assert.Equal(t, 1, vals[0])
	assert.Equal(t, 2, vals[1])
}

func TestWaitForValuesTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancelFn()

	sync := make(chan int)
	go func() {
		time.Sleep(1 * time.Millisecond)
		sync <- 1
		time.Sleep(5 * time.Millisecond)
		sync <- 2
	}()
	vals, err := WaitForValues(ctx, sync, 2)
	assert.Error(t, err)
	assert.Len(t, vals, 0)
}
