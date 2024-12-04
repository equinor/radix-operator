package v1

import (
	"github.com/equinor/radix-common/utils/pointers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// RadixProbe describes a health check to be performed against a container to determine whether it is
// alive or ready to receive traffic.
type RadixProbe struct {
	// The action taken to determine the health of a container
	RadixProbeHandler `json:",inline"`
	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
	// Number of seconds after which the probe times out.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +kubebuilder:validation:Minimum=1
	// +default=1
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
	// How often (in seconds) to perform the probe.
	// +kubebuilder:validation:Minimum=1
	// +default=10
	// +optional
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Must be 1 for liveness and startup.
	// +kubebuilder:validation:Minimum=1
	// +default=1
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// +kubebuilder:validation:Minimum=1
	// +default=3
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`

	// // Optional duration in seconds the pod needs to terminate gracefully upon probe failure.
	// // The grace period is the duration in seconds after the processes running in the pod are sent
	// // a termination signal and the time when the processes are forcibly halted with a kill signal.
	// // Set this value longer than the expected cleanup time for your process.
	// // If this value is nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this
	// // value overrides the value provided by the pod spec.
	// // Value must be non-negative integer. The value zero indicates stop immediately via
	// // the kill signal (no opportunity to shut down).
	// // This is a beta field and requires enabling ProbeTerminationGracePeriod feature gate.
	// // +kubebuilder:validation:Minimum=1
	// // +default=30
	// // +optional
	// TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

func (rp *RadixProbe) MapToCoreProbe() *corev1.Probe {
	if rp == nil {
		return nil
	}

	return &corev1.Probe{
		ProbeHandler:        rp.RadixProbeHandler.MapToCoreProbe(),
		InitialDelaySeconds: pointers.Val(rp.InitialDelaySeconds),
		TimeoutSeconds:      pointers.Val(rp.TimeoutSeconds),
		PeriodSeconds:       pointers.Val(rp.PeriodSeconds),
		SuccessThreshold:    pointers.Val(rp.SuccessThreshold),
		FailureThreshold:    pointers.Val(rp.FailureThreshold),
		// TerminationGracePeriodSeconds: rp.TerminationGracePeriodSeconds,
	}
}

// RadixProbeHandler defines a specific action that should be taken in a probe.
// One and only one of the fields must be specified.
type RadixProbeHandler struct {
	// Exec specifies the action to take.
	Exec *RadixProbeExecAction `json:"exec,omitempty"`
	// HTTPGet specifies the http request to perform.
	HTTPGet *RadixProbeHTTPGetAction `json:"httpGet,omitempty"`
	// TCPSocket specifies an action involving a TCP port.
	TCPSocket *RadixProbeTCPSocketAction `json:"tcpSocket,omitempty"`
	// GRPC specifies an action involving a GRPC port.
	GRPC *RadixProbeGRPCAction `json:"grpc,omitempty"`
}

func (p RadixProbeHandler) MapToCoreProbe() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec:      p.Exec.MapToCoreProbe(),
		HTTPGet:   p.HTTPGet.MapToCoreProbe(),
		TCPSocket: p.TCPSocket.MapToCoreProbe(),
		GRPC:      p.GRPC.MapToCoreProbe(),
	}
}

// RadixProbeHTTPGetAction describes an action based on HTTP Get requests.
type RadixProbeHTTPGetAction struct {
	// Path to access on the HTTP server.
	// +optional
	Path *string `json:"path,omitempty"`
	// port number to access on the container.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port"`
	// Host name to connect to, defaults to the pod IP. You probably want to set
	// "Host" in httpHeaders instead.
	// +optional
	Host *string `json:"host,omitempty"`
	// Scheme to use for connecting to the host.
	// Defaults to HTTP.
	// +optional
	// +kubebuilder:validation:Enum=HTTPS;HTTP
	Scheme *corev1.URIScheme `json:"scheme,omitempty"`
	// Custom headers to set in the request. HTTP allows repeated headers.
	// +optional
	// +listType=atomic
	HTTPHeaders []corev1.HTTPHeader `json:"httpHeaders,omitempty"`
}

func (a *RadixProbeHTTPGetAction) MapToCoreProbe() *corev1.HTTPGetAction {
	if a == nil {
		return nil
	}

	var port intstr.IntOrString
	if a.Port != nil {
		port = intstr.FromInt32(*a.Port)
	}

	return &corev1.HTTPGetAction{
		Path:        pointers.Val(a.Path),
		Port:        port,
		Host:        pointers.Val(a.Host),
		Scheme:      pointers.Val(a.Scheme),
		HTTPHeaders: a.HTTPHeaders,
	}
}

// RadixProbeExecAction describes a "run in container" action.
type RadixProbeExecAction struct {
	// Command is the command line to execute inside the container, the working directory for the
	// command  is root ('/') in the container's filesystem. The command is simply exec'd, it is
	// not run inside a shell, so traditional shell instructions ('|', etc) won't work. To use
	// a shell, you need to explicitly call out to that shell.
	// Exit status of 0 is treated as live/healthy and non-zero is unhealthy.
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty"`
}

func (a *RadixProbeExecAction) MapToCoreProbe() *corev1.ExecAction {
	if a == nil {
		return nil
	}

	return &corev1.ExecAction{
		Command: a.Command,
	}
}

// RadixProbeTCPSocketAction describes an action based on opening a socket
type RadixProbeTCPSocketAction struct {
	// port number to access on the container.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port"`
	// Optional: Host name to connect to, defaults to the pod IP.
	// +optional
	Host *string `json:"host,omitempty"`
}

func (a *RadixProbeTCPSocketAction) MapToCoreProbe() *corev1.TCPSocketAction {
	if a == nil {
		return nil
	}
	var port intstr.IntOrString
	if a.Port != nil {
		port = intstr.FromInt32(*a.Port)
	}

	return &corev1.TCPSocketAction{
		Port: port,
		Host: pointers.Val(a.Host),
	}
}

type RadixProbeGRPCAction struct {
	// Port number of the gRPC service.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Service is the name of the service to place in the gRPC HealthCheckRequest
	// (see https://github.com/grpc/grpc/blob/master/doc/health-checking.md).
	//
	// If this is not specified, the default behavior is defined by gRPC.
	// +optional
	// +default=""
	Service *string `json:"service"`
}

func (a *RadixProbeGRPCAction) MapToCoreProbe() *corev1.GRPCAction {
	if a == nil {
		return nil
	}

	return &corev1.GRPCAction{
		Port:    a.Port,
		Service: a.Service,
	}
}
