package libvirt

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// nodeIPOffset is the last-octet offset of the first Harvester node within the
// subnet (node 0 => .30, node 1 => .31, ...). Matches ipxe-examples layout.
const nodeIPOffset = 30

// GenerateMAC derives a deterministic, KVM-safe MAC address from the cluster
// name and node index. The 52:54 prefix is the QEMU/KVM locally-administered
// range; the remaining four octets come from a SHA-256 of the inputs so the
// same cluster+index always yields the same MAC and different inputs collide
// only astronomically rarely.
func GenerateMAC(clusterName string, nodeIndex int) string {
	sum := sha256.Sum256([]byte(clusterName + ":" + strconv.Itoa(nodeIndex)))
	return fmt.Sprintf("52:54:%02x:%02x:%02x:%02x", sum[0], sum[1], sum[2], sum[3])
}

// subnetBase returns the first three octets of a "A.B.C.0/24" subnet, e.g.
// "192.168.0" for "192.168.0.0/24".
func subnetBase(subnet string) (string, error) {
	cidr := subnet
	if i := strings.IndexByte(cidr, '/'); i >= 0 {
		cidr = cidr[:i]
	}
	octets := strings.Split(cidr, ".")
	if len(octets) != 4 {
		return "", fmt.Errorf("invalid network_subnet %q (want A.B.C.0/24)", subnet)
	}
	for _, o := range octets {
		n, err := strconv.Atoi(o)
		if err != nil || n < 0 || n > 255 {
			return "", fmt.Errorf("invalid network_subnet %q (want A.B.C.0/24)", subnet)
		}
	}
	return strings.Join(octets[:3], "."), nil
}

// nodeIP returns the IP for the Nth Harvester node in the subnet.
func nodeIP(base string, index int) string {
	return fmt.Sprintf("%s.%d", base, nodeIPOffset+index)
}

// ComputeTopology builds the planned VM layout for the given config. Node 0 is
// the cluster "create" node; all others "join". Returns an error if the subnet
// is malformed.
func ComputeTopology(c *Config) (*core.Topology, error) {
	base, err := subnetBase(c.NetworkSubnet)
	if err != nil {
		return nil, err
	}

	nodes := make([]core.HarvesterNode, 0, c.NodeCount)
	for i := 0; i < c.NodeCount; i++ {
		role := "join"
		if i == 0 {
			role = "create"
		}
		nodes = append(nodes, core.HarvesterNode{
			Name: fmt.Sprintf("%s-%d", c.NodePrefix, i),
			MAC:  GenerateMAC(c.ClusterName, i),
			IP:   nodeIP(base, i),
			Role: role,
		})
	}

	gateway := base + ".1" // libvirt network gateway = host side
	return &core.Topology{
		Gateway: gateway,
		Subnet:  c.NetworkSubnet,
		ISOURL:  fmt.Sprintf("http://%s:%d%s", gateway, ConfigServerPort, ISOURLPath),
		Nodes:   nodes,
	}, nil
}
