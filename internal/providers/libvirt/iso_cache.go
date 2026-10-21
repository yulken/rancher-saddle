package libvirt

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// grubCfgPath is the main GRUB config inside the Harvester ISO. The EFI
// grub.cfg just chainloads this one, so patching it covers both BIOS and UEFI.
const grubCfgPath = "/boot/grub2/grub.cfg"

// autoInstallSchema is bumped whenever the GRUB patch logic changes, so cached
// auto-install ISOs built by an older (possibly broken) patch are not reused.
const autoInstallSchema = "v4"

// AutoInstallISOPath returns the cached path of the repacked unattended-install
// ISO for a (version, configURL) pair. The configURL and patch schema are hashed
// into the name so a changed config server address OR a changed patch rebuilds
// the ISO rather than reusing a stale one.
func AutoInstallISOPath(mediaDir, version, configURL string) string {
	h := sha256.Sum256([]byte(autoInstallSchema + "\x00" + configURL))
	return filepath.Join(mediaDir, fmt.Sprintf("harvester-%s-autoinstall-%x.iso", version, h[:4]))
}

// RepackAutoInstallISO builds an unattended-install variant of srcISO at dstISO
// by patching its GRUB config so the installer boots automatically with
// harvester.install.automatic=true and the given config_url. No-op (returns nil)
// if dstISO already exists. Requires xorriso on the host.
func RepackAutoInstallISO(ctx context.Context, srcISO, dstISO, configURL string, logWriter io.Writer) error {
	if _, err := os.Stat(dstISO); err == nil {
		fmt.Fprintf(logWriter, "✓ auto-install ISO already built (%s)\n", filepath.Base(dstISO))
		return nil
	}
	if _, err := exec.LookPath("xorriso"); err != nil {
		return fmt.Errorf("xorriso is required to build the auto-install ISO: %w", err)
	}

	tmp, err := os.MkdirTemp("", "hv-grub-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Extract grub.cfg, patch it, and write it back into a copy of the ISO.
	local := filepath.Join(tmp, "grub.cfg")
	ex := exec.CommandContext(ctx, "xorriso", "-osirrox", "on", "-indev", srcISO, "-extract", grubCfgPath, local)
	if err := ex.Run(); err != nil {
		return fmt.Errorf("could not extract %s from %s: %w", grubCfgPath, srcISO, err)
	}
	if err := patchGrubCfg(local, configURL); err != nil {
		return err
	}

	fmt.Fprintf(logWriter, "Building unattended-install ISO (%s)...\n", filepath.Base(dstISO))
	cmd := exec.CommandContext(ctx, "xorriso",
		"-indev", srcISO, "-outdev", dstISO,
		"-boot_image", "any", "replay",
		"-update", local, grubCfgPath)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	if err := cmd.Run(); err != nil {
		os.Remove(dstISO)
		return fmt.Errorf("xorriso repack failed: %w", err)
	}
	return nil
}

// patchGrubCfg makes the Harvester installer boot unattended. The ISO's grub.cfg
// appends ${extra_iso_cmdline} to the kernel line, so we inject our install
// parameters through that variable (robust against the $linux/$initrd indirection
// it uses) and force the default entry to boot immediately (timeout 0).
func patchGrubCfg(path, configURL string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// ip=dhcp + rd.neednet=1 bring the NIC up early so the installer can reach the
	// config_url (otherwise it fails with "network is unreachable").
	// skipchecks=true bypasses the preflight hardware checks (CPU/RAM), which
	// otherwise block an automated install on a small test VM.
	args := fmt.Sprintf("ip=dhcp rd.neednet=1 harvester.install.automatic=true harvester.install.skipchecks=true harvester.install.config_url=%s", configURL)
	inject := fmt.Sprintf("set extra_iso_cmdline=%q\nset timeout=0\nset timeout_style=hidden", args)

	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines)+1)
	injected := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		// Drop the ISO's own timeout settings; we set our own below.
		if strings.HasPrefix(t, "set timeout=") || strings.HasPrefix(t, "set timeout_style=") {
			continue
		}
		out = append(out, ln)
		// Inject right after "set default=" so it runs before any menuentry.
		if !injected && strings.HasPrefix(t, "set default=") {
			out = append(out, inject)
			injected = true
		}
	}
	if !injected {
		out = append([]string{inject}, out...)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644)
}

// releaseBaseURL is where Harvester media is published. Overridable in tests.
var releaseBaseURL = "https://releases.rancher.com/harvester"

// ISOName returns the Harvester installer ISO filename for a version.
func ISOName(version string) string {
	return fmt.Sprintf("harvester-%s-amd64.iso", version)
}

// artifactNames returns the media files needed for the ISO-boot install flow.
// Only the installer ISO is required — the VMs boot it directly (as a CD-ROM)
// and self-install; no separate kernel/initrd/rootfs are needed.
func artifactNames(version string) []string {
	return []string{ISOName(version)}
}

// CacheDir returns the local cache directory for a Harvester version's media.
func CacheDir(projectRoot, version string) string {
	return filepath.Join(projectRoot, "harvester-isos", version)
}

// ListCachedVersions returns the Harvester versions that have a fully-present ISO
// in the shared cache under projectRoot, sorted descending (newest-looking
// first). A version counts as cached only when its .iso file exists.
func ListCachedVersions(projectRoot string) []string {
	base := filepath.Join(projectRoot, "harvester-isos")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		version := e.Name()
		iso := filepath.Join(base, version, fmt.Sprintf("harvester-%s-amd64.iso", version))
		if _, err := os.Stat(iso); err == nil {
			out = append(out, version)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

// EnsureISO makes sure all four Harvester media artifacts for the given version
// exist in the shared cache under projectRoot.
func EnsureISO(ctx context.Context, projectRoot, version string, logWriter io.Writer) error {
	return EnsureISOInDir(ctx, CacheDir(projectRoot, version), version, logWriter)
}

// EnsureISOInDir makes sure all four Harvester media artifacts for the given
// version exist in dir. Already-present files are skipped; missing files are
// downloaded from releases.rancher.com with progress written to logWriter. A
// partial download is removed on error so a retry starts clean. This is used
// both for the shared cache and for a user-supplied ISO directory (where it
// fills in any missing sibling artifacts).
func EnsureISOInDir(ctx context.Context, dir, version string, logWriter io.Writer) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir %q: %w", dir, err)
	}

	for _, name := range artifactNames(version) {
		dest := filepath.Join(dir, name)
		if _, err := os.Stat(dest); err == nil {
			fmt.Fprintf(logWriter, "✓ %s already cached\n", name)
			continue
		}
		url := fmt.Sprintf("%s/%s/%s", releaseBaseURL, version, name)
		if err := downloadFile(ctx, url, dest, name, logWriter); err != nil {
			os.Remove(dest) // clean up partial file
			return fmt.Errorf("failed to download %s: %w", name, err)
		}
	}
	return nil
}

// downloadFile streams url to dest, logging progress as it goes.
func downloadFile(ctx context.Context, url, dest, name string, logWriter io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s for %s", resp.Status, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	fmt.Fprintf(logWriter, "Downloading %s...\n", name)
	pw := &progressWriter{total: resp.ContentLength, name: name, w: logWriter}
	if _, err := io.Copy(out, io.TeeReader(resp.Body, pw)); err != nil {
		return err
	}
	fmt.Fprintf(logWriter, "✓ %s downloaded\n", name)
	return nil
}

// progressWriter logs download progress roughly every 50 MB.
type progressWriter struct {
	total      int64 // -1 if unknown
	downloaded int64
	lastLogged int64
	name       string
	w          io.Writer
}

const progressLogEvery = 50 << 20 // 50 MB

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.downloaded += int64(n)
	if p.downloaded-p.lastLogged >= progressLogEvery {
		p.lastLogged = p.downloaded
		if p.total > 0 {
			fmt.Fprintf(p.w, "  %s... %d MB / %d MB\n", p.name, p.downloaded>>20, p.total>>20)
		} else {
			fmt.Fprintf(p.w, "  %s... %d MB\n", p.name, p.downloaded>>20)
		}
	}
	return n, nil
}
