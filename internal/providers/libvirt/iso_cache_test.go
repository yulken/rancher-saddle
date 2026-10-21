package libvirt

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheDir(t *testing.T) {
	assert.Equal(t,
		filepath.Join("/proj", "harvester-isos", "v1.4.1"),
		CacheDir("/proj", "v1.4.1"))
}

func TestEnsureISO_SkipsExisting(t *testing.T) {
	root := t.TempDir()
	version := "v1.4.1"
	dir := CacheDir(root, version)
	require.NoError(t, os.MkdirAll(dir, 0755))
	// Pre-create all four artifacts so nothing should be downloaded.
	for _, name := range artifactNames(version) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}

	// Point the base URL at a server that fails the test if hit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected download request: %s", r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Cleanup(swapReleaseURL(srv.URL))

	var buf bytes.Buffer
	assert.NoError(t, EnsureISO(context.Background(), root, version, &buf))
	assert.Contains(t, buf.String(), "already cached")
}

func TestEnsureISO_DownloadError(t *testing.T) {
	root := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Cleanup(swapReleaseURL(srv.URL))

	var buf bytes.Buffer
	err := EnsureISO(context.Background(), root, "v1.4.1", &buf)
	assert.Error(t, err)

	// Partial file must be cleaned up.
	first := filepath.Join(CacheDir(root, "v1.4.1"), artifactNames("v1.4.1")[0])
	_, statErr := os.Stat(first)
	assert.True(t, os.IsNotExist(statErr), "partial download should be removed")
}

func TestEnsureISO_DownloadsMissing(t *testing.T) {
	root := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-media"))
	}))
	defer srv.Close()
	t.Cleanup(swapReleaseURL(srv.URL))

	var buf bytes.Buffer
	require.NoError(t, EnsureISO(context.Background(), root, "v1.4.1", &buf))
	for _, name := range artifactNames("v1.4.1") {
		data, err := os.ReadFile(filepath.Join(CacheDir(root, "v1.4.1"), name))
		require.NoError(t, err)
		assert.Equal(t, "fake-media", string(data))
	}
}

// swapReleaseURL points downloads at a test server and returns a restore func.
func swapReleaseURL(url string) func() {
	orig := releaseBaseURL
	releaseBaseURL = url
	return func() { releaseBaseURL = orig }
}

func TestListCachedVersions(t *testing.T) {
	root := t.TempDir()

	// v1.4.1: full (has ISO) -> counts. v1.3.2: dir only, no ISO -> excluded.
	full := CacheDir(root, "v1.4.1")
	require.NoError(t, os.MkdirAll(full, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(full, "harvester-v1.4.1-amd64.iso"), []byte("x"), 0644))

	partial := CacheDir(root, "v1.3.2")
	require.NoError(t, os.MkdirAll(partial, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(partial, "harvester-v1.3.2-initrd-amd64"), []byte("x"), 0644))

	got := ListCachedVersions(root)
	assert.Equal(t, []string{"v1.4.1"}, got)
}

func TestListCachedVersions_NoCache(t *testing.T) {
	assert.Nil(t, ListCachedVersions(t.TempDir()))
}

func TestAutoInstallISOPath(t *testing.T) {
	a := AutoInstallISOPath("/media", "v1.4.1", "http://192.168.0.1:8181/config")
	b := AutoInstallISOPath("/media", "v1.4.1", "http://192.168.0.1:8181/config")
	assert.Equal(t, a, b, "same inputs -> same path")

	// Different config URL -> different ISO name (so a stale one isn't reused).
	c := AutoInstallISOPath("/media", "v1.4.1", "http://10.0.0.1:8181/config")
	assert.NotEqual(t, a, c)
	assert.Contains(t, a, "harvester-v1.4.1-autoinstall-")
}

func TestPatchGrubCfg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "grub.cfg")
	// Mirrors the real Harvester grub.cfg: $linux variable + ${extra_iso_cmdline}.
	orig := "set default=0\nset timeout=10\nset timeout_style=menu\n" +
		"menuentry \"Harvester Installer\" {\n" +
		"  $linux ($root)/boot/kernel cdroot root=live:CDLABEL=COS_LIVE ${extra_iso_cmdline}\n" +
		"  $initrd ($root)/boot/initrd\n}\n"
	require.NoError(t, os.WriteFile(path, []byte(orig), 0644))

	require.NoError(t, patchGrubCfg(path, "http://192.168.0.1:8181/config"))

	s := func() string { b, _ := os.ReadFile(path); return string(b) }()

	// Injects the install args (incl. early DHCP) through extra_iso_cmdline and
	// boots immediately.
	assert.Contains(t, s, "set extra_iso_cmdline=")
	assert.Contains(t, s, "ip=dhcp")
	assert.Contains(t, s, "harvester.install.automatic=true")
	assert.Contains(t, s, "harvester.install.skipchecks=true")
	assert.Contains(t, s, "harvester.install.config_url=http://192.168.0.1:8181/config")
	assert.Contains(t, s, "set timeout=0")
	assert.NotContains(t, s, "set timeout=10")
	// Kernel/menuentry lines are left untouched.
	assert.Contains(t, s, "${extra_iso_cmdline}")
}
