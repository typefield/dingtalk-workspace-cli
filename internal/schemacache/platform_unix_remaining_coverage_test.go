//go:build (darwin && arm64) || (linux && amd64)

package schemacache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"golang.org/x/sys/unix"
)

type failFstatIO struct{ unixIO }

func (failFstatIO) fstat(int, *unix.Stat_t) error { return errors.New("fstat") }

type toggleCloseIO struct {
	unixIO
	fail atomic.Bool
}

func (t *toggleCloseIO) close(fd int) error {
	if t.fail.Load() {
		_ = t.unixIO.close(fd)
		return errors.New("close")
	}
	return t.unixIO.close(fd)
}

type mutateSizeIO struct {
	unixIO
	flip atomic.Bool
	n    atomic.Int32
}

func (m *mutateSizeIO) fstat(fd int, stat *unix.Stat_t) error {
	if err := m.unixIO.fstat(fd, stat); err != nil {
		return err
	}
	if m.flip.Load() && m.n.Add(1) >= 2 {
		stat.Size++
	}
	return nil
}

type xorPayloadIO struct {
	unixIO
	xor atomic.Bool
}

func (x *xorPayloadIO) pread(fd int, p []byte, offset int64) (int, error) {
	n, err := x.unixIO.pread(fd, p, offset)
	if x.xor.Load() && n > 0 && offset >= HeaderSize {
		p[0] ^= 1
	}
	return n, err
}

type notDirStatIO struct {
	unixIO
	once atomic.Bool
}

func (n *notDirStatIO) fstat(fd int, stat *unix.Stat_t) error {
	if err := n.unixIO.fstat(fd, stat); err != nil {
		return err
	}
	if n.once.CompareAndSwap(false, true) {
		stat.Mode = unix.S_IFREG | 0o600
	}
	return nil
}

func TestCrossPlatformCoverageUnixSystemCacheAndIOErrors(t *testing.T) {
	base := privateTestBase(t)
	oldUser, oldIO := userCacheDir, platformIO
	userCacheDir = func() (string, error) { return base, nil }
	platformIO = realUnixIO{}
	t.Cleanup(func() { userCacheDir, platformIO = oldUser, oldIO })

	first, err := Open("official")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	t.Setenv("DWS_SCHEMA_CACHE_DIR", "")
	testseam.Swap(t, &systemCacheBase, func() string { return base })
	shared, err := Open("official")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shared.Close() })

	counters := &Counters{}
	if _, _, err := openCacheDirectory("/", "abcd", counters, realUnixIO{}, true, true); err == nil {
		t.Fatal("root cache directory")
	}
	if _, _, err := openCacheDirectory("/usr/dws-schema-coverage-must-not-exist", "abcd", counters, realUnixIO{}, false, false); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("missing ancestry = %v", err)
	}

	ops := failFstatIO{unixIO: realUnixIO{}}
	if _, _, err := openCacheDirectory(base, "abcd", counters, ops, true, false); err == nil {
		t.Fatal("fstat ancestry")
	}

	closer := &toggleCloseIO{unixIO: realUnixIO{}}
	cache, _, identity := openTestCache(t, closer)
	meta := testArtifact(KindMeta, []byte("meta"))
	registry := testArtifact(KindRegistry, []byte("registry-payload"))
	payloads := testArtifact(KindPayloads, []byte("payload-bytes"))
	if err := cache.Publish(identity, registry, meta, payloads); err != nil {
		t.Fatal(err)
	}
	closer.fail.Store(true)
	if _, err := cache.ReadMeta(identity, meta.Expectation); err == nil {
		t.Fatal("close after ReadMeta")
	}
	closer.fail.Store(false)

	mut := &mutateSizeIO{unixIO: realUnixIO{}}
	cache2, _, identity2 := openTestCache(t, mut)
	meta2 := testArtifact(KindMeta, []byte("meta-two"))
	reg2 := testArtifact(KindRegistry, []byte("registry-two"))
	pay2 := testArtifact(KindPayloads, []byte("payload-two"))
	if err := cache2.Publish(identity2, reg2, meta2, pay2); err != nil {
		t.Fatal(err)
	}
	mut.flip.Store(true)
	if _, err := cache2.OpenRegistry(identity2, reg2.Expectation); err == nil {
		t.Fatal("registry changed during open")
	}

	xor := &xorPayloadIO{unixIO: realUnixIO{}}
	cache3, _, identity3 := openTestCache(t, xor)
	meta3 := testArtifact(KindMeta, []byte("meta-three"))
	reg3 := testArtifact(KindRegistry, []byte("registry-three-bytes"))
	if err := cache3.Publish(identity3, reg3, meta3); err != nil {
		t.Fatal(err)
	}
	handle, err := cache3.OpenRegistry(identity3, reg3.Expectation)
	if err != nil {
		t.Fatal(err)
	}
	xor.xor.Store(true)
	if err := handle.ValidateAggregate(); err == nil {
		t.Fatal("aggregate digest")
	}
	_ = handle.Close()
	_ = handle.Close()
	if err := handle.ValidateAggregate(); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed aggregate = %v", err)
	}

	path := filepath.Join(cache.Directory(), metaFileName)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := append([]byte(nil), original...)
	corrupted[0] ^= 0xff
	if err := os.WriteFile(path, corrupted, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.ReadMeta(identity, meta.Expectation); err == nil {
		t.Fatal("bad magic envelope")
	}
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := cache.WriteArtifact(identity, Artifact{}); err == nil {
		t.Fatal("invalid envelope")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForRetry(ctx, time.Now().Add(time.Second)); err == nil {
		t.Fatal("canceled wait")
	}
	if err := waitForRetry(context.Background(), time.Now().Add(-time.Second)); !errors.Is(err, ErrLockTimeout) {
		t.Fatalf("expired wait = %v", err)
	}
	if err := waitForRetry(context.Background(), time.Now().Add(50*time.Millisecond)); err != nil && !errors.Is(err, ErrLockTimeout) {
		t.Fatalf("retry wait = %v", err)
	}
	lock := &localLock{token: make(chan struct{})}
	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	if err := takeLocalLock(ctx, lock, time.Now().Add(time.Second)); err == nil {
		t.Fatal("canceled local lock")
	}
	ready := &localLock{token: make(chan struct{}, 1)}
	ready.token <- struct{}{}
	if err := takeLocalLock(context.Background(), ready, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("immediate local lock = %v", err)
	}

	closedCache, _, _ := openTestCache(t, nil)
	_ = closedCache.Close()
	if _, err := closedCache.AcquireLock(context.Background(), time.Millisecond); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed acquire = %v", err)
	}
	if _, err := (&unixCache{ops: realUnixIO{}, counters: &Counters{}}).acquire(context.Background(), -time.Second); err == nil {
		t.Fatal("acquire without dir")
	}

	closer.fail.Store(false)
	held, err := cache.AcquireLock(context.Background(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	closer.fail.Store(true)
	if err := held.Release(); err == nil {
		t.Fatal("lock close")
	}
	closer.fail.Store(false)

	file, err := os.CreateTemp(base, "notdir")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(file.Name()) })
	_ = file.Close()
	fd, err := unix.Open(file.Name(), unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	if err := validateOwnedDirectory(fd, &Counters{}, realUnixIO{}, false); err == nil {
		t.Fatal("file is not a directory")
	}
	sharedDir := filepath.Join(base, "shared-mode")
	if err := os.Mkdir(sharedDir, 0o777); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(sharedDir, 0o777)
	dirfd, err := unix.Open(sharedDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(dirfd) })
	if err := validateOwnedDirectory(dirfd, &Counters{}, realUnixIO{}, true); err == nil {
		t.Fatal("world-writable shared dir")
	}
	if err := validateCacheFile(fileState{mode: unix.S_IFREG | 0o666, nlink: 1, uid: uint32(unix.Geteuid())}, true); err == nil {
		t.Fatal("world-writable shared file")
	}
}

func TestCrossPlatformCoverageUnixSharedDirectoryAndRangeErrors(t *testing.T) {
	notDir := &notDirStatIO{unixIO: realUnixIO{}}
	base := privateTestBase(t)
	counters := &Counters{}
	if _, _, err := openCacheDirectory(base, "abcd", counters, notDir, false, true); err == nil {
		t.Fatal("owned directory not a dir")
	}

	xor := &xorPayloadIO{unixIO: realUnixIO{}}
	cache, _, identity := openTestCache(t, xor)
	meta := testArtifact(KindMeta, []byte("meta"))
	registry := testArtifact(KindRegistry, []byte("0123456789abcdef0123456789abcdef"))
	if err := cache.Publish(identity, registry, meta); err != nil {
		t.Fatal(err)
	}
	handle, err := cache.OpenRegistry(identity, registry.Expectation)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	xor.xor.Store(true)
	sum := registry.Expectation.EncodedSHA256
	if _, err := handle.ReadRange(RangeDescriptor{Offset: 0, Length: 4, SHA256: sum}); err == nil {
		t.Fatal("range digest")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForRetry(ctx, time.Now().Add(20*time.Millisecond)); err == nil {
		t.Fatal("canceled short wait")
	}
}
