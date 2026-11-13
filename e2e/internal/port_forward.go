package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForward forwards a random local port to port on the pod until ctx is cancelled, and returns the local address
func PortForward(ctx context.Context, cfg *rest.Config, namespace, pod string, port int) (string, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create clientset: %w", err)
	}
	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create SPDY round tripper: %w", err)
	}
	reqURL := clientset.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(pod).SubResource("portforward").URL()
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL)

	stop, ready := make(chan struct{}), make(chan struct{})
	forwarder, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, []string{fmt.Sprintf("0:%d", port)}, stop, ready, io.Discard, os.Stderr)
	if err != nil {
		return "", fmt.Errorf("failed to create port forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- forwarder.ForwardPorts() }()
	go func() {
		<-ctx.Done()
		close(stop)
	}()

	select {
	case <-ready:
	case err := <-errCh:
		return "", fmt.Errorf("failed to forward port %d of pod %s/%s: %w", port, namespace, pod, err)
	case <-ctx.Done():
		return "", ctx.Err()
	}

	ports, err := forwarder.GetPorts()
	if err != nil {
		return "", fmt.Errorf("failed to get forwarded ports: %w", err)
	}
	return fmt.Sprintf("127.0.0.1:%d", ports[0].Local), nil
}
