package internal

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// GitServerImageName is the local image name for the e2e in-cluster git server.
	GitServerImageName = "local-kind-repo/git-server"

	gitServerNamespace = "git-server"
	gitServerName      = "git-server"
	// GitServerHost is the hostname the clone workload connects to. Because RadixRegistration
	// CloneURLs are locked to git@github.com, the e2e cluster's CoreDNS rewrites this host to the
	// in-cluster git server service (see ConfigureCoreDNSGithubRewrite).
	GitServerHost = "github.com"

	gitServerServiceFQDN  = gitServerName + "." + gitServerNamespace + ".svc.cluster.local"
	gitServerHostKeyField = "ssh_host_ed25519_key"

	// gitServerReposDir is the host directory holding the source repositories served by the git
	// server. Each immediate subtree ending in ".git" becomes a bare repository served under that
	// path, matching the RadixRegistration CloneURLs (e.g. git@github.com:equinor/queue-order-test.git).
	gitServerReposDir = "testdata/gitserver/repos"

	// archPlaceholder in any committed file is replaced with the host runtime architecture so the
	// build and runtime pods schedule on the local kind node. runtime.GOARCH already uses the same
	// values as Radix (amd64/arm64), and the kind node shares the host architecture.
	archPlaceholder = "{{ARCH}}"
)

// BuildGitServerImage builds the e2e git server image from e2e/testdata/gitserver/git-server.Dockerfile.
func BuildGitServerImage(ctx context.Context, imageTag string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fullImageName := fmt.Sprintf("%s:%s", GitServerImageName, imageTag)
	log.Info().Msgf("Building git server image %s...", fullImageName)

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", fullImageName, "-f", "testdata/gitserver/git-server.Dockerfile", "testdata/gitserver")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build git server image %s: %w", fullImageName, err)
	}

	log.Info().Msgf("Successfully built %s", fullImageName)
	return nil
}

// DeployGitServer deploys the in-cluster SSH git server and returns the known_hosts entry for the
// GitServerHost host alias. The server serves equinor/queue-order-test.git over SSH and accepts any
// client public key, so the operator-generated deploy key authenticates without prior provisioning.
func DeployGitServer(ctx context.Context, c client.Client, imageTag string) (knownHosts string, err error) {
	// Reuse an existing host key when the kind cluster is reused, so the known_hosts entry stays
	// stable across runs; only generate a new key on first deployment.
	hostKeyPEM, knownHostsEntry, err := getOrGenerateHostKey(ctx, c)
	if err != nil {
		return "", fmt.Errorf("failed to obtain git server host key: %w", err)
	}

	// Build the repository archive on the host; the server only extracts and serves it.
	repoArchive, err := buildRepoArchive(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build git server repository archive: %w", err)
	}

	image := fmt.Sprintf("%s:%s", GitServerImageName, imageTag)
	if err := applyGitServerResources(ctx, c, image, hostKeyPEM, repoArchive); err != nil {
		return "", err
	}

	// Restart the deployment so a reused cluster picks up the current image/host key, then wait
	// until it is available.
	if err := restartDeployment(ctx, c, gitServerNamespace, gitServerName); err != nil {
		return "", err
	}
	if err := waitForDeploymentAvailable(ctx, c, gitServerNamespace, gitServerName, 3*time.Minute); err != nil {
		return "", fmt.Errorf("git server deployment not ready: %w", err)
	}

	return knownHostsEntry, nil
}

// getOrGenerateHostKey returns the host key PEM and the matching known_hosts entry, reusing the key
// already stored in the host key secret when present.
func getOrGenerateHostKey(ctx context.Context, c client.Client) (privatePEM []byte, knownHostsEntry string, err error) {
	existing := &corev1.Secret{}
	err = c.Get(ctx, client.ObjectKey{Namespace: gitServerNamespace, Name: gitServerName + "-hostkey"}, existing)
	switch {
	case err == nil && len(existing.Data[gitServerHostKeyField]) > 0:
		pemBytes := existing.Data[gitServerHostKeyField]
		entry, deriveErr := knownHostsFromPrivateKey(pemBytes)
		if deriveErr != nil {
			return nil, "", deriveErr
		}
		return pemBytes, entry, nil
	case err != nil && !k8serrors.IsNotFound(err):
		return nil, "", err
	default:
		return generateHostKey()
	}
}

// knownHostsFromPrivateKey derives the GitServerHost known_hosts entry from an SSH private key PEM.
func knownHostsFromPrivateKey(pemBytes []byte) (string, error) {
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse git server host key: %w", err)
	}
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	return fmt.Sprintf("%s %s\n", GitServerHost, authorizedKey), nil
}

// generateHostKey generates an ed25519 SSH host key and returns the OpenSSH-format private key PEM
// together with a known_hosts line scoped to GitServerHost.
func generateHostKey() (privatePEM []byte, knownHostsEntry string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}

	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, "", err
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, "", err
	}

	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
	knownHostsEntry = fmt.Sprintf("%s %s\n", GitServerHost, authorizedKey)
	return pem.EncodeToMemory(block), knownHostsEntry, nil
}

// buildRepoArchive creates one bare git repository (with main, dev and prod branches) for every
// source repository under gitServerReposDir and returns them as a gzipped tar archive. Each
// immediate subtree ending in ".git" is committed and served at the same relative path, so the
// server can serve them without knowing their contents. Any archPlaceholder token in committed
// files is replaced with the host architecture.
func buildRepoArchive(ctx context.Context) ([]byte, error) {
	repoPaths, err := discoverRepos(gitServerReposDir)
	if err != nil {
		return nil, err
	}
	if len(repoPaths) == 0 {
		return nil, fmt.Errorf("no repositories found under %s", gitServerReposDir)
	}

	tmpDir, err := os.MkdirTemp("", "git-server-repo-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for _, relRepoPath := range repoPaths {
		if err := buildBareRepo(ctx, tmpDir, relRepoPath, filepath.Join(gitServerReposDir, relRepoPath)); err != nil {
			return nil, err
		}
	}

	return tarGz(filepath.Join(tmpDir, "srv"))
}

// discoverRepos returns the repository paths (ending in ".git", relative to reposDir) found under
// reposDir.
func discoverRepos(reposDir string) ([]string, error) {
	var repos []string
	walkErr := filepath.Walk(reposDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && strings.HasSuffix(path, ".git") {
			rel, relErr := filepath.Rel(reposDir, path)
			if relErr != nil {
				return relErr
			}
			repos = append(repos, filepath.ToSlash(rel))
			return filepath.SkipDir
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("failed to discover repositories under %s: %w", reposDir, walkErr)
	}
	return repos, nil
}

// buildBareRepo copies the source repository at srcDir into a working tree, commits it on the main,
// dev and prod branches, and clones it as a bare repository at tmpDir/srv/<relRepoPath>. The
// archPlaceholder token in any copied file is replaced with the host architecture.
func buildBareRepo(ctx context.Context, tmpDir, relRepoPath, srcDir string) error {
	work := filepath.Join(tmpDir, "work", relRepoPath)
	if err := copyRepoFiles(srcDir, work); err != nil {
		return err
	}

	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=e2e", "GIT_AUTHOR_EMAIL=e2e@example.com",
		"GIT_COMMITTER_NAME=e2e", "GIT_COMMITTER_EMAIL=e2e@example.com",
	)
	runGit := func(dir string, args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, string(out))
		}
		return nil
	}

	if err := runGit(work, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	if err := runGit(work, "add", "-A"); err != nil {
		return err
	}
	if err := runGit(work, "commit", "-q", "-m", "init"); err != nil {
		return err
	}
	if err := runGit(work, "branch", "dev"); err != nil {
		return err
	}
	if err := runGit(work, "branch", "prod"); err != nil {
		return err
	}

	bare := filepath.Join(tmpDir, "srv", relRepoPath)
	if err := os.MkdirAll(filepath.Dir(bare), 0o755); err != nil {
		return err
	}
	return runGit(tmpDir, "clone", "-q", "--bare", work, bare)
}

// copyRepoFiles copies all files from srcDir into dstDir, replacing archPlaceholder with the host
// architecture in their contents.
func copyRepoFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = bytes.ReplaceAll(content, []byte(archPlaceholder), []byte(runtime.GOARCH))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, content, info.Mode().Perm())
	})
}

// tarGz returns the contents of root packaged as a gzipped tar archive, with paths relative to root.
func tarGz(root string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		_, err = io.Copy(tw, file)
		return err
	})
	if walkErr != nil {
		return nil, walkErr
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func applyGitServerResources(ctx context.Context, c client.Client, image string, hostKeyPEM, repoArchive []byte) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gitServerNamespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, ns, func() error { return nil }); err != nil {
		return fmt.Errorf("failed to apply git server namespace: %w", err)
	}

	hostKeySecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gitServerName + "-hostkey", Namespace: gitServerNamespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, hostKeySecret, func() error {
		hostKeySecret.Type = corev1.SecretTypeOpaque
		hostKeySecret.Data = map[string][]byte{gitServerHostKeyField: hostKeyPEM}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to apply git server host key secret: %w", err)
	}

	repoConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: gitServerName + "-repo", Namespace: gitServerNamespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, repoConfigMap, func() error {
		repoConfigMap.BinaryData = map[string][]byte{"repo.tar.gz": repoArchive}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to apply git server repo config map: %w", err)
	}

	labels := map[string]string{"app": gitServerName}
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: gitServerName, Namespace: gitServerNamespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, deployment, func() error {
		replicas := int32(1)
		hostKeyMode := int32(0o400)
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		deployment.Spec.Template.ObjectMeta.Labels = labels
		deployment.Spec.Template.Spec = corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            gitServerName,
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports:           []corev1.ContainerPort{{ContainerPort: 22}},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "hostkey", MountPath: "/etc/gitserver-keys", ReadOnly: true},
						{Name: "repo", MountPath: "/etc/gitserver-repo", ReadOnly: true},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "hostkey",
					VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
						SecretName:  hostKeySecret.Name,
						DefaultMode: &hostKeyMode,
					}},
				},
				{
					Name: "repo",
					VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: repoConfigMap.Name},
					}},
				},
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to apply git server deployment: %w", err)
	}

	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: gitServerName, Namespace: gitServerNamespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, service, func() error {
		service.Spec.Selector = labels
		service.Spec.Ports = []corev1.ServicePort{{
			Name:       "ssh",
			Port:       22,
			TargetPort: intstr.FromInt32(22),
		}}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to apply git server service: %w", err)
	}

	return nil
}

// ConfigureCoreDNSGithubRewrite makes the e2e cluster resolve GitServerHost to the in-cluster git
// server by adding a CoreDNS rewrite rule. It is idempotent.
func ConfigureCoreDNSGithubRewrite(ctx context.Context, c client.Client) error {
	corednsCm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: "coredns"}, corednsCm); err != nil {
		return fmt.Errorf("failed to get coredns config map: %w", err)
	}

	corefile := corednsCm.Data["Corefile"]
	rewriteRule := fmt.Sprintf("rewrite name %s %s", GitServerHost, gitServerServiceFQDN)
	if strings.Contains(corefile, rewriteRule) {
		return nil
	}

	marker := ".:53 {"
	idx := strings.Index(corefile, marker)
	if idx == -1 {
		return fmt.Errorf("unexpected Corefile format, %q block not found", marker)
	}
	insertAt := idx + len(marker)
	updated := corefile[:insertAt] + "\n    " + rewriteRule + corefile[insertAt:]

	corednsCm.Data["Corefile"] = updated
	if err := c.Update(ctx, corednsCm); err != nil {
		return fmt.Errorf("failed to update coredns config map: %w", err)
	}

	// Restart CoreDNS so the new rule takes effect immediately rather than waiting for the reload.
	if err := restartDeployment(ctx, c, "kube-system", "coredns"); err != nil {
		return err
	}
	return waitForDeploymentAvailable(ctx, c, "kube-system", "coredns", 2*time.Minute)
}

func restartDeployment(ctx context.Context, c client.Client, namespace, name string) error {
	// Use a merge patch instead of get+update so we don't conflict with concurrent status updates
	// from the deployment controller.
	patch := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"e2e/restartedAt":%q}}}}}`, time.Now().Format(time.RFC3339Nano)))
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	if err := c.Patch(ctx, deployment, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return fmt.Errorf("failed to restart deployment %s/%s: %w", namespace, name, err)
	}
	return nil
}

func waitForDeploymentAvailable(ctx context.Context, c client.Client, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		deployment := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
			return false, nil
		}
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		s := deployment.Status
		return s.ObservedGeneration >= deployment.Generation &&
			desired > 0 &&
			s.UpdatedReplicas == desired &&
			s.AvailableReplicas == desired &&
			s.ReadyReplicas == desired, nil
	})
}
