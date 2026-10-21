package libvirt

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigServer_ServesByIP(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config-node0.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("mode: create\n"), 0644))

	// Requests from the test come from 127.0.0.1.
	srv, err := StartConfigServer("127.0.0.1:0", map[string]string{"127.0.0.1": cfgPath}, "", io.Discard)
	require.NoError(t, err)
	defer srv.Close()

	resp, err := http.Get("http://" + srv.Addr() + "/config")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "mode: create\n", string(body))
}

func TestConfigServer_ServesISO(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "harvester.iso")
	require.NoError(t, os.WriteFile(iso, []byte("ISO-BYTES"), 0644))

	srv, err := StartConfigServer("127.0.0.1:0", map[string]string{}, iso, io.Discard)
	require.NoError(t, err)
	defer srv.Close()

	resp, err := http.Get("http://" + srv.Addr() + ISOURLPath)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "ISO-BYTES", string(body))
}

func TestConfigServer_404ForUnknownIP(t *testing.T) {
	srv, err := StartConfigServer("127.0.0.1:0", map[string]string{"10.0.0.5": "/nope"}, "", io.Discard)
	require.NoError(t, err)
	defer srv.Close()

	resp, err := http.Get("http://" + srv.Addr() + "/config")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
