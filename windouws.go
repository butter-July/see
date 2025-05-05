//go:build windows
// +build windows

package main

import (
	"syscall"
	"unsafe"
)


var (
	modUser32                    = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowTextLengthW     = modUser32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW           = modUser32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")

	modKernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleFileNameExW = modKernel32.NewProc("GetModuleFileNameExW")

	modPsapi                     = syscall.NewLazyDLL("psapi.dll")
	procGetProcessImageFileNameW = modPsapi.NewProc("GetProcessImageFileNameW")
)

const (
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
)


func getForegroundWindow() uintptr {
	ret, _, _ := procGetForegroundWindow.Call()
	return ret
}
func getWindowTextLength(hwnd uintptr) int {
	ret, _, _ := procGetWindowTextLengthW.Call(hwnd)
	return int(ret)
}


func getWindowText(hwnd uintptr, str *uint16, maxCount int) int {
	ret, _, _ := procGetWindowTextW.Call(
		hwnd,
		uintptr(unsafe.Pointer(str)),
		uintptr(maxCount))
	return int(ret)
}


func getWindowThreadProcessId(hwnd uintptr, pid *uint32) uint32 {
	ret, _, _ := procGetWindowThreadProcessId.Call(
		hwnd,
		uintptr(unsafe.Pointer(pid)))
	return uint32(ret)
}

func getProcessImageFileName(handle syscall.Handle, outPath *uint16, size uint32) (err error) {
	r1, _, e1 := procGetProcessImageFileNameW.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(outPath)),
		uintptr(size),
	)
	if r1 == 0 {
		if e1 != nil {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func getForegroundWindowInfo() string {
	hwnd := getForegroundWindow()
	if hwnd == 0 {
		return "Unknown"
	}

	
	length := getWindowTextLength(hwnd) + 1
	buf := make([]uint16, length)
	getWindowText(hwnd, &buf[0], length)
	windowTitle := syscall.UTF16ToString(buf)

	
	var pid uint32
	getWindowThreadProcessId(hwnd, &pid)

	
	handle, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, pid)
	if err != nil {
		return windowTitle
	}
	defer syscall.CloseHandle(handle)

	
	var exePath [syscall.MAX_PATH]uint16
	err = getProcessImageFileName(handle, &exePath[0], syscall.MAX_PATH)
	if err != nil {
		return windowTitle
	}

	
	exeName := syscall.UTF16ToString(exePath[:])
	for i := len(exeName) - 1; i >= 0; i-- {
		if exeName[i] == '\\' {
			exeName = exeName[i+1:]
			break
		}
	}

	
	if exeName != "" {
			for i := len(exeName) - 1; i >= 0; i-- {
			if exeName[i] == '.' {
				exeName = exeName[:i]
				break
			}
		}
		return exeName
	}
	return windowTitle
}
