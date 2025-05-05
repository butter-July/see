//go:build !windows
// +build !windows

package main

import (
	"os/exec"
)

// Linux版本的活动窗口检测
func getForegroundWindowInfo() string {
	// 尝试使用xdotool获取活动窗口（如果是图形环境）
	cmd := exec.Command("xdotool", "getwindowfocus", "getwindowname")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return string(output)
	}

	// 如果无法获取窗口信息，返回一个默认值
	hostname, err := exec.Command("hostname").Output()
	if err == nil {
		return "Terminal on " + string(hostname)
	}

	return "Linux Terminal"
}
