package launchlib_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/palantir/go-java-launcher/launchlib"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	cgroupContent = []byte(`12:memory:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
11:blkio:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
10:cpu,cpuacct:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
9:hugetlb:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
8:freezer:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
7:pids:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
6:perf_event:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
5:rdma:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
4:net_cls,net_prio:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
3:cpuset:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
2:devices:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
1:name=systemd:/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151
0::/5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151`)

	mountInfoContent = []byte(`5087 3462 0:337 / / rw,relatime master:945 - overlay overlay rw,lowerdir=/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/618/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/617/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/616/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/615/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/614/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/15/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/14/fs,upperdir=/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/619/fs,workdir=/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/619/work,xino=off
5088 5087 0:338 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
5089 5087 0:339 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
5090 5089 0:356 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
5091 5089 0:304 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
5092 5087 0:333 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
5093 5092 0:433 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - tmpfs tmpfs rw,mode=755
5094 5093 0:30 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/systemd ro,nosuid,nodev,noexec,relatime master:9 - cgroup cgroup rw,xattr,name=systemd
5095 5093 0:33 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/devices ro,nosuid,nodev,noexec,relatime master:15 - cgroup cgroup rw,devices
5096 5093 0:34 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/cpuset ro,nosuid,nodev,noexec,relatime master:16 - cgroup cgroup rw,cpuset
5097 5093 0:35 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/net_cls,net_prio ro,nosuid,nodev,noexec,relatime master:17 - cgroup cgroup rw,net_cls,net_prio
5098 5093 0:36 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/rdma ro,nosuid,nodev,noexec,relatime master:18 - cgroup cgroup rw,rdma
5133 5093 0:37 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/perf_event ro,nosuid,nodev,noexec,relatime master:19 - cgroup cgroup rw,perf_event
5134 5093 0:38 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/pids ro,nosuid,nodev,noexec,relatime master:20 - cgroup cgroup rw,pids
5135 5093 0:39 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/freezer ro,nosuid,nodev,noexec,relatime master:21 - cgroup cgroup rw,freezer
5136 5093 0:40 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/hugetlb ro,nosuid,nodev,noexec,relatime master:22 - cgroup cgroup rw,hugetlb
5137 5093 0:41 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/cpu,cpuacct ro,nosuid,nodev,noexec,relatime master:23 - cgroup cgroup rw,cpu,cpuacct
5138 5093 0:42 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/blkio ro,nosuid,nodev,noexec,relatime master:24 - cgroup cgroup rw,blkio
5139 5093 0:43 /5f371271ccf0fa6b567f2cec7054b449931d48c603fadc31487214897c206151 /sys/fs/cgroup/memory ro,nosuid,nodev,noexec,relatime master:25 - cgroup cgroup rw,memory
3463 5088 0:338 /bus /proc/bus ro,nosuid,nodev,noexec,relatime - proc proc rw
3464 5088 0:338 /fs /proc/fs ro,nosuid,nodev,noexec,relatime - proc proc rw
3465 5088 0:338 /irq /proc/irq ro,nosuid,nodev,noexec,relatime - proc proc rw
3466 5088 0:338 /sys /proc/sys ro,nosuid,nodev,noexec,relatime - proc proc rw
3467 5088 0:338 /sysrq-trigger /proc/sysrq-trigger ro,nosuid,nodev,noexec,relatime - proc proc rw
3468 5088 0:434 / /proc/acpi ro,relatime - tmpfs tmpfs ro
3469 5088 0:339 /null /proc/kcore rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
3470 5088 0:339 /null /proc/keys rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
3471 5088 0:339 /null /proc/timer_list rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
3472 5088 0:339 /null /proc/sched_debug rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
3473 5088 0:435 / /proc/scsi ro,relatime - tmpfs tmpfs ro
3474 5092 0:436 / /sys/firmware ro,relatime - tmpfs tmpfs ro`)

	badMountInfoContent = []byte(`5087 3462 0:337 / / rw,relatime master:945 - overlay overlay rw,lowerdir=/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/618/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/617/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/616/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/615/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/614/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/15/fs:/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/14/fs,upperdir=/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/619/fs,workdir=/var/lib/container-runtime/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/619/work,xino=off
5088 5087 0:338 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
5089 5087 0:339 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
5090 5089 0:356 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
5091 5089 0:304 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
5092 5087 0:333 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
5093 5092 0:433 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - tmpfs tmpfs rw,mode=755
3463 5088 0:338 /bus /proc/bus ro,nosuid,nodev,noexec,relatime - proc proc rw
3464 5088 0:338 /fs /proc/fs ro,nosuid,nodev,noexec,relatime - proc proc rw
3465 5088 0:338 /irq /proc/irq ro,nosuid,nodev,noexec,relatime - proc proc rw
3466 5088 0:338 /sys /proc/sys ro,nosuid,nodev,noexec,relatime - proc proc rw
3467 5088 0:338 /sysrq-trigger /proc/sysrq-trigger ro,nosuid,nodev,noexec,relatime - proc proc rw
3468 5088 0:434 / /proc/acpi ro,relatime - tmpfs tmpfs ro
3473 5088 0:435 / /proc/scsi ro,relatime - tmpfs tmpfs ro
3474 5092 0:436 / /sys/firmware ro,relatime - tmpfs tmpfs ro`)

	CGroupTestFS = fstest.MapFS{
		"proc/self/cgroup": &fstest.MapFile{
			Data: cgroupContent,
		},
		"proc/self/mountinfo": &fstest.MapFile{
			Data: mountInfoContent,
		},
	}
)

func TestCGroupPather_GetPath(t *testing.T) {
	for _, test := range []struct {
		name          string
		filesystem    fs.FS
		moduleName    string
		expectedPath  string
		expectedError error
	}{
		{
			name:          "fails when unable to read self cgroup",
			filesystem:    fstest.MapFS{},
			expectedError: errors.New("failed to open cgroup file"),
		},
		{
			name: "fails when unable to read self mountinfo",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: cgroupContent,
				},
			},
			expectedError: errors.New("failed to open mountinfo file"),
		},
		{
			name: "fails when unable to parse mountinfo for cpu cgroup location",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: cgroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: badMountInfoContent,
				},
			},
			expectedError: errors.New("unable to find cpu cgroup mount path"),
		},
		{
			name:         "returns correct path",
			moduleName:   "cpu",
			filesystem:   CGroupTestFS,
			expectedPath: "/sys/fs/cgroup/cpu",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pather := launchlib.NewCGroupV1Pather(test.filesystem)
			cgroupPath, err := pather.GetPath(test.moduleName)
			if test.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedPath, cgroupPath)
		})
	}
}
