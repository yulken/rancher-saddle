package libvirt

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// ConfigServerPort is the port the built-in config server listens on. The VMs
// fetch their Harvester install config from http://<gateway>:<port>/config
// during the unattended install.
const ConfigServerPort = 8181

// ISOURLPath is the path the config server serves the Harvester ISO on (used as
// install.iso_url in the generated config).
const ISOURLPath = "/harvester.iso"

// ConfigServer is a tiny HTTP server that serves the ISO-auto-install inputs over
// the libvirt network: each node's install config at "/config" (routed by source
// IP, so node 0 gets "create" and the rest "join") and the Harvester ISO at
// "/harvester.iso" (install.iso_url). No PXE, DHCP or TFTP involved.
type ConfigServer struct {
	srv     *http.Server
	ln      net.Listener
	byIP    map[string]string // source IP -> config file path
	isoPath string            // path to the Harvester ISO served at /harvester.iso
	log     io.Writer
}

// StartConfigServer begins serving on addr (e.g. "0.0.0.0:8181"). byIP maps each
// node's IP to its rendered config file path; isoPath is the Harvester ISO to
// serve for install.iso_url.
func StartConfigServer(addr string, byIP map[string]string, isoPath string, logWriter io.Writer) (*ConfigServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("config server listen on %s: %w", addr, err)
	}
	cs := &ConfigServer{byIP: byIP, isoPath: isoPath, log: logWriter, ln: ln}
	mux := http.NewServeMux()
	mux.HandleFunc(ISOURLPath, cs.handleISO)
	mux.HandleFunc("/", cs.handle)
	cs.srv = &http.Server{Handler: mux}
	go func() { _ = cs.srv.Serve(ln) }()
	return cs, nil
}

// handleISO serves the Harvester ISO for install.iso_url.
func (cs *ConfigServer) handleISO(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(cs.log, "[config-server] serving ISO to %s\n", r.RemoteAddr)
	http.ServeFile(w, r, cs.isoPath)
}

func (cs *ConfigServer) handle(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	path, ok := cs.byIP[ip]
	if !ok {
		fmt.Fprintf(cs.log, "[config-server] no config mapped for %s\n", ip)
		http.Error(w, "no config for host", http.StatusNotFound)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(cs.log, "[config-server] served %s to %s\n", filepath.Base(path), ip)
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(data)
}

// Addr returns the actual listen address (useful when the port was 0 in tests).
func (cs *ConfigServer) Addr() string { return cs.ln.Addr().String() }

// Close stops the server.
func (cs *ConfigServer) Close() error { return cs.srv.Close() }
