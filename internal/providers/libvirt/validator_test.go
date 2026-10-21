package libvirt

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// withStubs swaps the package-level exec/stat hooks for the duration of a test.
func withStubs(t *testing.T, look func(string) (string, error), run func(string, ...string) ([]byte, error), stat func(string) (os.FileInfo, error)) {
	t.Helper()
	origLook, origRun, origStat := lookPath, commandRunner, statFn
	lookPath, commandRunner, statFn = look, run, stat
	t.Cleanup(func() { lookPath, commandRunner, statFn = origLook, origRun, origStat })
}

func TestCheckPrerequisites_AllPresent(t *testing.T) {
	withStubs(t,
		func(string) (string, error) { return "/usr/bin/tool", nil }, // virsh/qemu-img/xorriso found
		func(name string, args ...string) ([]byte, error) { return []byte("active"), nil },
		func(string) (os.FileInfo, error) { return nil, nil },
	)
	assert.NoError(t, CheckPrerequisites())
}

func TestCheckPrerequisites_AllMissing(t *testing.T) {
	withStubs(t,
		func(string) (string, error) { return "", errors.New("not found") },
		func(name string, args ...string) ([]byte, error) { return []byte("inactive"), errors.New("fail") },
		func(string) (os.FileInfo, error) { return nil, errors.New("no kvm") },
	)
	err := CheckPrerequisites()
	assert.Error(t, err)
	// Collects all failures, not just the first.
	assert.Contains(t, err.Error(), "virsh not found")
	assert.Contains(t, err.Error(), "qemu-img not found")
	assert.Contains(t, err.Error(), "xorriso not found")
	assert.Contains(t, err.Error(), "libvirtd is not active")
	assert.Contains(t, err.Error(), "/dev/kvm")
}

func TestCheckPrerequisites_OnlyXorrisoMissing(t *testing.T) {
	withStubs(t,
		func(name string) (string, error) {
			if name == "xorriso" {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + name, nil
		},
		func(name string, args ...string) ([]byte, error) { return []byte("active"), nil },
		func(string) (os.FileInfo, error) { return nil, nil },
	)
	err := CheckPrerequisites()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "xorriso not found")
	assert.NotContains(t, err.Error(), "virsh not found")
	assert.NotContains(t, err.Error(), "libvirtd is not active")
}
