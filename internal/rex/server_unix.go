//go:build darwin || linux

package rex

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// lockFile locks f for this process, or fails at once when another holds
// it.
func lockFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }

// detached starts the server in a session of its own, which outlives the
// app's.
func detached() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

// hostInfo describes this machine; it may take a second.
func hostInfo() HostInfo {
	h := HostInfo{}
	if u, err := user.Current(); err == nil {
		h.User = u.Username
		h.Home = u.HomeDir
	}
	out := func(name string, args ...string) string {
		b, err := exec.Command(name, args...).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	if runtime.GOOS == "darwin" {
		h.Name = out("scutil", "--get", "ComputerName")
		h.OS = "macOS " + out("sw_vers", "-productVersion")
		h.Chip = sysctlString("machdep.cpu.brand_string")
		h.Memory = sysctlUint64("hw.memsize")
		var hw struct {
			Data []struct {
				MachineName string `json:"machine_name"`
				ChipType    string `json:"chip_type"`
			} `json:"SPHardwareDataType"`
		}
		if json.Unmarshal([]byte(out("system_profiler", "SPHardwareDataType", "-json", "-detailLevel", "mini")), &hw) == nil && len(hw.Data) > 0 {
			h.Model = hw.Data[0].MachineName
			if hw.Data[0].ChipType != "" {
				h.Chip = hw.Data[0].ChipType
			}
		}
		if h.Model == "" {
			h.Model = sysctlString("hw.model")
		}
	} else {
		h.Name, _ = os.Hostname()
		h.OS = runtime.GOOS
		if b, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				if v, ok := strings.CutPrefix(l, "PRETTY_NAME="); ok {
					h.OS, _ = strconv.Unquote(v)
				}
			}
		}
		h.Model = "Linux"
		if runtime.GOOS == "linux" {
			linuxHost(&h)
		}
	}
	if h.Name == "" {
		h.Name, _ = os.Hostname()
		h.Name = strings.TrimSuffix(h.Name, ".local")
	}
	return h
}

// linuxHost reads what Linux tells of the machine: its model, processor,
// memory, and whether it has a battery.
func linuxHost(h *HostInfo) {
	read := func(p string) string {
		b, _ := os.ReadFile(p)
		return strings.TrimSpace(string(b))
	}
	vendor, product := read("/sys/class/dmi/id/sys_vendor"), read("/sys/class/dmi/id/product_version")
	if product == "" || strings.EqualFold(product, "none") || strings.Contains(strings.ToLower(product), "o.e.m") {
		product = read("/sys/class/dmi/id/product_name")
	}
	if product != "" && !strings.Contains(strings.ToLower(product), "to be filled") {
		if vendor != "" && !strings.HasPrefix(strings.ToLower(product), strings.ToLower(vendor)) {
			product = vendor + " " + product
		}
		h.Model = product
	}
	for _, l := range strings.Split(read("/proc/cpuinfo"), "\n") {
		if k, v, ok := strings.Cut(l, ":"); ok && strings.TrimSpace(k) == "model name" {
			h.Chip = strings.Join(strings.Fields(v), " ")
			break
		}
	}
	for _, l := range strings.Split(read("/proc/meminfo"), "\n") {
		if v, ok := strings.CutPrefix(l, "MemTotal:"); ok {
			if kb, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 10, 64); err == nil {
				h.Memory = kb << 10
			}
		}
	}
	if m, _ := filepath.Glob("/sys/class/power_supply/BAT*"); len(m) > 0 {
		h.Portable = true
	}
}
