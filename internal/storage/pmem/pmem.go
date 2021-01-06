// +build linux

package pmem

import (
	"context"
	"fmt"
	"os"

	"github.com/Microsoft/opengcs/internal/oc"
	"github.com/Microsoft/opengcs/internal/shell"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/sys/unix"
)

// Test dependencies
var (
	osMkdirAll  = os.MkdirAll
	osRemoveAll = os.RemoveAll
	unixMount   = unix.Mount
)

// Mount mounts the pmem device at `/dev/pmem<device>` to `target`.
//
// `target` will be created. On mount failure the created `target` will be
// automatically cleaned up.
//
// Note: For now the platform only supports readonly pmem that is assumed to be
// `ext4`.
func Mount(ctx context.Context, device uint32, target string) (err error) {
	_, span := trace.StartSpan(ctx, "pmem::Mount")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()

	span.AddAttributes(
		trace.Int64Attribute("device", int64(device)),
		trace.StringAttribute("target", target))

	var debugStr = fmt.Sprintf("pmem::Mount device %s target %s\n", device, target)
	shell.WriteOut(debugStr)

	if err := osMkdirAll(target, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			osRemoveAll(target)
		}
	}()

	source := fmt.Sprintf("/dev/pmem%d", device)

	// use dumpe2fs to extract the number of blocks and block size of the pmem device we are mounting
	pmemDevName := fmt.Sprintf("/dev/pmem%d", device)

	blockInfo := []string{"Block count:", "Block size:"}
	fsAnswers, err := shell.ShellOutWithResults(ctx, "/sbin/dumpe2fs", pmemDevName, blockInfo, true)

	if err != nil {
		shell.WriteError(err, "failed to get block size/count from pmem device %d\n", device)
		return errors.Wrapf(err, "failed to get block size/count from pmem device %d\n", device)
	}

	blockSize := fsAnswers["Block size:"]
	blockCount := fsAnswers["Block count:"]

	// TODO only do this is we need intergity checking on the these pmem provided filesystems.
	//      likely via some attribute passed from the host

	// The basic plan here is to use veritysetup format to make a merkle tree in /tmp
	// and then veritysetup create to create a mapped device in /dev/mapper which is then
	// mounted.
	//
	// For example, for /dev/pmem9 we create a /tmp/hash9 and a mapped device /dev/mapper/verity9
	// then when we mount we mount /dev/mapper/verity9 on target rather than mount /dev/pmem9 there.
	// Eventually we may cache the merkle tree to avoid the cost in time here.

	// make this compare device > 0 if you find the UVM fails to boot and you need to look at the log in /tmp
	if device >= 0 {
		/*
			This is how the veritysetup command makes a hash tree on, in this case /tmp/hashes7

			C:\ContainerPlat>shimdiag exec k8 veritysetup format /dev/pmem7 /tmp/hashes7
			VERITY header information for /tmp/hashes.img
			UUID:                   286b6abe-dc96-41d4-9d6b-8ceef55e5e62
			Hash type:              1
			Data blocks:            1048576
			Data block size:        4096
			Hash block size:        4096
			Hash algorithm:         sha256
			Salt:                   523b54cb9f2aefe307cfcba25e1df8581fb76f28dda7c61bea9377853e56bac4
			Root hash:              0391a02dc3fcd5a62ef3cc9fd7248c073c2f188156564cf2728497eecd69cb50
		*/
		// Use a salt of zero so as to obtain a repeatable and so verifyable root hash
		formatArgs := fmt.Sprintf("format --salt=0000000000000000000000000000000000000000000000000000000000000000 --data-block-size=%s --data-blocks=%s /dev/pmem%d /tmp/hash%d", blockSize, blockCount, device, device)

		hashInfo := []string{"Root hash:"} // we want to extract the generated root hash to pass to the next step
		hashAnswers, verityFormatErr := shell.ShellOutWithResults(ctx, "/usr/sbin/veritysetup", formatArgs, hashInfo, true)
		if verityFormatErr != nil {
			shell.WriteError(verityFormatErr, "failed to veritysetup format device %d\n", device)
			return errors.Wrapf(verityFormatErr, "failed to veritysetup format device %d\n", device)
		}

		hash := hashAnswers["Root hash:"]
		span.AddAttributes(trace.StringAttribute("hash", hash))

		// create the device mapper entry veritysetup create verity7 /dev/loop1 /tmp/hashes.img 0391a02dc3fcd5a62ef3cc9fd7248c073c2f188156564cf2728497eecd69cb50

		emptyInfo := []string{} // no results from the next line, TODO - use just ShellOut?
		createArgs := fmt.Sprintf("create verity%d /dev/pmem%d /tmp/hash%d %s", device, device, device, hash)
		emptyAnswers, verityCreateErr := shell.ShellOutWithResults(ctx, "/usr/sbin/veritysetup", createArgs, emptyInfo, true)
		if verityCreateErr != nil {
			shell.WriteError(verityFormatErr, "failed to veritysetup create device %d\n", device)
			return errors.Wrapf(verityFormatErr, "failed to veritysetup create device %d\n", device)
		}

		// emptyAnswers is unused.
		_ = emptyAnswers

		source = fmt.Sprintf("/dev/mapper/verity%d", device)
	}

	flags := uintptr(unix.MS_RDONLY)
	if err := unixMount(source, target, "ext4", flags, "noload"); err != nil {
		shell.WriteError(err, "failed to mount pmem device %s onto %s", source, target)
		return errors.Wrapf(err, "failed to mount pmem device %s onto %s", source, target)
	}
	return nil
}
