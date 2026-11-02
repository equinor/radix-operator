// Command fakeblobcsi is a fake CSI node plugin for e2e tests. It registers with the name of the
// Azure Blob CSI driver (blob.csi.azure.com), so statically provisioned Radix PersistentVolumes can
// be mounted in a kind cluster without Azure.
//
// Each volume is a directory on the node, named after the volume handle, that is bind mounted into
// the Pod. Pods that use the same PersistentVolume share data. The file fake-blob-csi.json in the
// volume contains the volume handle, volume attributes and mount options that kubelet passed to
// the driver, so tests can verify them.
//
// The driver does not advertise STAGE_UNSTAGE_VOLUME, so kubelet never calls NodeStageVolume and
// does not need the nodeStageSecretRef secret.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	volumeInfoFileName = "fake-blob-csi.json"
	version            = "0.1.0"
)

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// VolumeInfo is written to fake-blob-csi.json in the root of each volume
type VolumeInfo struct {
	VolumeID         string            `json:"volumeId"`
	VolumeContext    map[string]string `json:"volumeContext"`
	MountFlags       []string          `json:"mountFlags"`
	AccessMode       string            `json:"accessMode"`
	Readonly         bool              `json:"readonly"`
	NodePublishCount int               `json:"nodePublishCount"`
}

type driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer

	name    string
	nodeID  string
	dataDir string

	mu sync.Mutex
}

func main() {
	endpoint := flag.String("endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	name := flag.String("drivername", "blob.csi.azure.com", "CSI driver name")
	nodeID := flag.String("nodeid", "", "Node ID")
	dataDir := flag.String("datadir", "/var/lib/fake-blob-csi", "Directory where volume data is stored")
	flag.Parse()

	if *nodeID == "" {
		log.Fatal("--nodeid is required")
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	socketPath := strings.TrimPrefix(*endpoint, "unix://")
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove socket %s: %v", socketPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		log.Fatalf("failed to create socket dir: %v", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", socketPath, err)
	}

	d := &driver{name: *name, nodeID: *nodeID, dataDir: *dataDir}
	server := grpc.NewServer(grpc.UnaryInterceptor(logRequests))
	csi.RegisterIdentityServer(server, d)
	csi.RegisterNodeServer(server, d)
	log.Printf("fake CSI driver %s %s serving on %s, node %s", d.name, version, socketPath, d.nodeID)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func logRequests(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		log.Printf("%s failed: %v", info.FullMethod, err)
	} else if !strings.HasSuffix(info.FullMethod, "/Probe") {
		log.Printf("%s: %v", info.FullMethod, req)
	}
	return resp, err
}

func (d *driver) GetPluginInfo(context.Context, *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{Name: d.name, VendorVersion: version}, nil
}

func (d *driver) GetPluginCapabilities(context.Context, *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{}, nil
}

func (d *driver) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}

func (d *driver) NodeGetInfo(context.Context, *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{NodeId: d.nodeID}, nil
}

func (d *driver) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func (d *driver) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.GetVolumeId() == "" || req.GetTargetPath() == "" || req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "volume id, target path and volume capability are required")
	}
	if req.GetVolumeCapability().GetMount() == nil {
		return nil, status.Error(codes.InvalidArgument, "only mount volumes are supported")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	targetPath := req.GetTargetPath()
	mounted, err := isMountPoint(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check mount point %s: %v", targetPath, err)
	}
	if mounted {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	sourcePath := filepath.Join(d.dataDir, unsafeChars.ReplaceAllString(req.GetVolumeId(), "_"))
	if err := os.MkdirAll(sourcePath, 0o777); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create volume dir %s: %v", sourcePath, err)
	}
	// Allow any non-root user in the Pod to write to the volume
	if err := os.Chmod(sourcePath, 0o777); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to chmod volume dir %s: %v", sourcePath, err)
	}
	if err := writeVolumeInfo(sourcePath, req); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write volume info: %v", err)
	}

	if err := os.MkdirAll(targetPath, 0o750); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create target path %s: %v", targetPath, err)
	}
	if err := unix.Mount(sourcePath, targetPath, "", unix.MS_BIND, ""); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to bind mount %s to %s: %v", sourcePath, targetPath, err)
	}
	if req.GetReadonly() {
		if err := unix.Mount("", targetPath, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
			_ = unix.Unmount(targetPath, 0)
			return nil, status.Errorf(codes.Internal, "failed to remount %s read-only: %v", targetPath, err)
		}
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *driver) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req.GetVolumeId() == "" || req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id and target path are required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	targetPath := req.GetTargetPath()
	mounted, err := isMountPoint(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check mount point %s: %v", targetPath, err)
	}
	if mounted {
		if err := unix.Unmount(targetPath, 0); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to unmount %s: %v", targetPath, err)
		}
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to remove %s: %v", targetPath, err)
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func writeVolumeInfo(sourcePath string, req *csi.NodePublishVolumeRequest) error {
	infoPath := filepath.Join(sourcePath, volumeInfoFileName)
	info := VolumeInfo{}
	if data, err := os.ReadFile(infoPath); err == nil {
		_ = json.Unmarshal(data, &info)
	}
	info.VolumeID = req.GetVolumeId()
	info.VolumeContext = req.GetVolumeContext()
	info.MountFlags = req.GetVolumeCapability().GetMount().GetMountFlags()
	info.AccessMode = req.GetVolumeCapability().GetAccessMode().GetMode().String()
	info.Readonly = req.GetReadonly()
	info.NodePublishCount++

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(infoPath, data, 0o644)
}

// isMountPoint returns true if path is a mount point, by looking it up in /proc/self/mountinfo
func isMountPoint(path string) (bool, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	cleanPath := filepath.Clean(path)
	for line := range strings.SplitSeq(string(data), "\n") {
		// Field 5 is the mount point, ref https://man7.org/linux/man-pages/man5/proc_pid_mountinfo.5.html
		fields := strings.Fields(line)
		if len(fields) >= 5 && unescapeMountInfo(fields[4]) == cleanPath {
			return true, nil
		}
	}
	return false, nil
}

// unescapeMountInfo decodes octal escapes (e.g. \040 for space) used in /proc/self/mountinfo
func unescapeMountInfo(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			var c byte
			if _, err := fmt.Sscanf(s[i+1:i+4], "%03o", &c); err == nil {
				b.WriteByte(c)
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
