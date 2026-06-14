package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

	backup := self + ".bak"
	os.Remove(backup)

	if err := os.Rename(newPath, self); err != nil {
		data, rErr := os.ReadFile(newPath)
		if rErr != nil {
			return fmt.Errorf("read new binary: %w", rErr)
		}
		if wErr := os.WriteFile(self, data, 0755); wErr != nil {
			return fmt.Errorf("write self: %w", wErr)
		}
	}

	os.Remove(backup)
	return nil
}

func RestartService(name string) error {
	out, err := exec.Command("systemctl", "--user", "restart", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart %s: %w\n%s", name, err, out)
	}
	return nil
}
