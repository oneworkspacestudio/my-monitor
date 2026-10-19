//go:build windows

package app

import (
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"

	"strings"
	"unsafe"
)

func appID() string { return apphost.ID() }
func validService(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_-. ", c)) {
			return false
		}
	}
	return true
}
func serviceRunning(name string) bool {
	if !validService(name) {
		return false
	}
	scm, e := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if e != nil {
		return false
	}
	defer windows.CloseServiceHandle(scm)
	p, e := windows.UTF16PtrFromString(name)
	if e != nil {
		return false
	}
	service, e := windows.OpenService(scm, p, windows.SERVICE_QUERY_STATUS)
	if e != nil {
		return false
	}
	defer windows.CloseServiceHandle(service)
	var status windows.SERVICE_STATUS_PROCESS
	var needed uint32
	e = windows.QueryServiceStatusEx(service, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafe.Pointer(&status)), uint32(unsafe.Sizeof(status)), &needed)
	return e == nil && status.CurrentState == uint32(svc.Running)
}
func metrics(root string) map[string]any {
	type memory struct {
		Length, Load                                                             uint32
		Total, Avail, TotalPage, AvailPage, TotalVirtual, AvailVirtual, Extended uint64
	}
	m := memory{}
	m.Length = uint32(unsafe.Sizeof(m))
	ok, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&m)))
	out := map[string]any{}
	if ok != 0 {
		out["memoryPercent"] = m.Load
		out["memoryTotal"] = m.Total
		out["memoryAvailable"] = m.Avail
	}
	p, e := windows.UTF16PtrFromString(root)
	if e == nil {
		var available, total, free uint64
		e = windows.GetDiskFreeSpaceEx(p, &available, &total, &free)
		if e == nil {
			out["diskAvailable"] = available
			out["diskTotal"] = total
		}
	}
	var idle, kernel, user windows.Filetime
	if ok, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemTimes").Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); ok != 0 {
		out["cpuIdle100ns"] = uint64(idle.HighDateTime)<<32 | uint64(idle.LowDateTime)
		out["cpuTotal100ns"] = (uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)) + (uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime))
	}
	return out
}
