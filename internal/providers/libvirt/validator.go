package libvirt

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// commandRunner runs a command and returns its combined output. It is a package
// variable so tests can stub out external command execution.
var commandRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// lookPath is a package variable wrapping exec.LookPath so tests can stub it.
var lookPath = exec.LookPath

// statFn wraps os.Stat so tests can stub the /dev/kvm check.
var statFn = os.Stat

// CheckPrerequisites verifies the host has everything needed to run a local
// Harvester install via libvirt/KVM. It collects ALL failures and returns them
// together, so the user can fix everything in one pass instead of discovering
// missing tools one at a time.
func CheckPrerequisites() error {
	var problems []string

	// virsh (libvirt client) in PATH
	if _, err := lookPath("virsh"); err != nil {
		problems = append(problems, "virsh not found in PATH (install libvirt-client)")
	}

	// qemu-img for creating the install disks
	if _, err := lookPath("qemu-img"); err != nil {
		problems = append(problems, "qemu-img not found in PATH (install qemu-tools/qemu-utils)")
	}

	// xorriso for building the unattended-install ISO
	if _, err := lookPath("xorriso"); err != nil {
		problems = append(problems, "xorriso not found in PATH (install xorriso)")
	}

	// libvirtd running
	if out, err := commandRunner("systemctl", "is-active", "libvirtd"); err != nil || strings.TrimSpace(string(out)) != "active" {
		problems = append(problems, "libvirtd is not active (run: sudo systemctl start libvirtd)")
	}

	// KVM available
	if _, err := statFn("/dev/kvm"); err != nil {
		problems = append(problems, "/dev/kvm not available (enable hardware virtualization / load KVM modules)")
	}

	if len(problems) > 0 {
		return fmt.Errorf("libvirt prerequisites not met:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}
