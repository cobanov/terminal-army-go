package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cobanov/terminal-army-go/internal/version"
	"github.com/spf13/cobra"
)

const defaultUpdateRepo = "cobanov/terminal-army-go"

type updateOptions struct {
	repo       string
	target     string
	installDir string
	force      bool
}

func newUpdateCmd() *cobra.Command {
	var opts updateOptions
	opts.repo = defaultUpdateRepo
	opts.target = "latest"

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the tarmy binary from GitHub releases",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.target, "to", opts.target, "release tag to install, or latest")
	cmd.Flags().StringVar(&opts.installDir, "install-dir", "", "directory to install into (defaults to the current binary directory when writable)")
	cmd.Flags().StringVar(&opts.repo, "repo", opts.repo, "GitHub repo slug to update from")
	cmd.Flags().BoolVar(&opts.force, "force", false, "reinstall even when the selected version is already installed")
	return cmd
}

func runUpdate(cmd *cobra.Command, opts updateOptions) error {
	client := &http.Client{Timeout: 45 * time.Second}
	out := cmd.OutOrStdout()

	tag := opts.target
	if tag == "" {
		tag = "latest"
	}
	if tag == "latest" {
		resolved, err := resolveLatestRelease(client, opts.repo)
		if err != nil {
			return err
		}
		tag = resolved
	}
	if tag == "" {
		return errors.New("release tag is empty")
	}
	if version.Version == tag && !opts.force {
		fmt.Fprintf(out, "tarmy is already at %s\n", tag)
		return nil
	}

	platform, err := updatePlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	archiveName := releaseArchiveName(tag, platform)
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", opts.repo, tag)

	installDir, err := chooseUpdateInstallDir(opts.installDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	fmt.Fprintf(out, "==> version : %s\n", tag)
	fmt.Fprintf(out, "==> platform: %s\n", platform)
	fmt.Fprintf(out, "==> destdir : %s\n", installDir)

	tmp, err := os.MkdirTemp("", "tarmy-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, archiveName)
	archiveURL := baseURL + "/" + archiveName
	fmt.Fprintf(out, "==> downloading %s\n", archiveURL)
	if err := downloadFile(client, archiveURL, archivePath); err != nil {
		return fmt.Errorf("download release archive: %w", err)
	}

	checksumsPath := filepath.Join(tmp, "checksums.txt")
	if err := downloadFile(client, baseURL+"/checksums.txt", checksumsPath); err == nil {
		expected, err := checksumForArchive(checksumsPath, archiveName)
		if err != nil {
			return err
		}
		if expected != "" {
			actual, err := sha256File(archivePath)
			if err != nil {
				return err
			}
			if actual != expected {
				return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
			}
			fmt.Fprintf(out, "ok  checksum verified (%s)\n", expected)
		}
	} else {
		fmt.Fprintln(out, "warn no checksums.txt published; skipping verify")
	}

	extracted, err := extractBinaryFromTarGz(archivePath, tmp, "tarmy")
	if err != nil {
		return err
	}
	dest := filepath.Join(installDir, "tarmy")
	if err := installBinary(extracted, dest); err != nil {
		return err
	}
	fmt.Fprintf(out, "ok  installed tarmy %s to %s\n", tag, dest)
	return nil
}

func updatePlatform(goos, goarch string) (string, error) {
	if goos != "darwin" && goos != "linux" {
		return "", fmt.Errorf("unsupported operating system: %s (need linux or darwin)", goos)
	}
	switch goarch {
	case "amd64", "arm64":
		return goos + "_" + goarch, nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s (need amd64 or arm64)", goarch)
	}
}

func releaseArchiveName(tag, platform string) string {
	return fmt.Sprintf("tarmy_%s_%s.tar.gz", strings.TrimPrefix(tag, "v"), platform)
}

func chooseUpdateInstallDir(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	exe, err := os.Executable()
	if err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
			exe = resolved
		}
		if filepath.Base(exe) == "tarmy" && dirWritable(filepath.Dir(exe)) {
			return filepath.Dir(exe), nil
		}
	}
	if dirWritable("/usr/local/bin") || os.Geteuid() == 0 {
		return "/usr/local/bin", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func dirWritable(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	f, err := os.CreateTemp(dir, ".tarmy-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func resolveLatestRelease(client *http.Client, repo string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tarmy-updater")
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var payload struct {
				TagName string `json:"tag_name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return "", err
			}
			if payload.TagName != "" {
				return payload.TagName, nil
			}
		}
	}

	redirectClient := *client
	redirectClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	req, err = http.NewRequest(http.MethodGet, "https://github.com/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "tarmy-updater")
	resp, err = redirectClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusTemporaryRedirect {
		return "", fmt.Errorf("resolve latest release: github returned %s", resp.Status)
	}
	tag := tagFromLatestRedirect(resp.Header.Get("Location"))
	if tag == "" {
		return "", errors.New("resolve latest release: could not parse GitHub redirect")
	}
	return tag, nil
}

func tagFromLatestRedirect(location string) string {
	location = strings.TrimSpace(location)
	if location == "" {
		return ""
	}
	parts := strings.Split(strings.TrimRight(location, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "tag" && parts[i+1] != "" {
			return parts[i+1]
		}
	}
	return ""
}

func downloadFile(client *http.Client, url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "tarmy-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s returned %s", url, resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func checksumForArchive(path, archive string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == archive || filepath.Base(name) == archive {
			return fields[0], nil
		}
	}
	return "", nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func extractBinaryFromTarGz(archivePath, destDir, binaryName string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		outPath := filepath.Join(destDir, binaryName)
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return outPath, nil
	}
	return "", fmt.Errorf("archive did not contain a %q binary", binaryName)
}

func installBinary(src, dest string) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".tarmy-update-*")
	if err != nil {
		return fmt.Errorf("create temp binary in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	in, err := os.Open(src)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		_ = in.Close()
		_ = tmp.Close()
		return err
	}
	if err := in.Close(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("install %s: %w", dest, err)
	}
	return nil
}
