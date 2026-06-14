package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func AssetName(base string) string {
	return fmt.Sprintf("%s-%s-%s", base, runtime.GOOS, runtime.GOARCH)
}

func DownloadLatest(owner, repo, asset string) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/%s", owner, repo, asset)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "rsk-update-*")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("write temp: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("chmod: %w", err)
	}

	return tmp.Name(), nil
}

func VersionOf(path string) (string, error) {
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return "", fmt.Errorf("run version: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return "", fmt.Errorf("empty version output")
	}
	return parts[len(parts)-1], nil
}

func ReplaceSelf(newPath string) error {
	self, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return fmt.Errorf("read /proc/self/exe: %w", err)
	}

	data, err := os.ReadFile(newPath)
	if err != nil {
		return fmt.Errorf("read new binary: %w", err)
	}

	// Create temp on same filesystem as target so rename works atomically
	tmp, err := os.CreateTemp(filepath.Dir(self), ".rsk-update-*")
	if err != nil {
		return fmt.Errorf("temp on target dir: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, self); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

func RestartService(name string) error {
	out, err := exec.Command("systemctl", "--user", "restart", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart %s: %w\n%s", name, err, out)
	}
	return nil
}
