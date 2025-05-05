package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// 定义Windows API常量
const (
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
)

// Config 结构体定义
type Config struct {
	Username        string
	Port            int
	RefreshInterval int
}

// 默认配置
var config = Config{
	Username:        "zz",
	Port:            8080,
	RefreshInterval: 1000,
}

type UserStatus struct {
	Username  string
	UsingApp  string
	Timestamp string
	mutex     sync.RWMutex
}

var currentStatus = UserStatus{
	Username: config.Username,
	UsingApp: "starting....",
}

func (us *UserStatus) UpdateStatus(app string) {
	us.mutex.Lock()
	defer us.mutex.Unlock()
	us.UsingApp = app
	us.Timestamp = time.Now().Format("2006-01-02 15:04:05")
}

func (us *UserStatus) GetStatus() (string, string, string) {
	us.mutex.RLock()
	defer us.mutex.RUnlock()
	return us.Username, us.UsingApp, us.Timestamp
}

// Windows API 函数
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

// 获取前台窗口句柄
func getForegroundWindow() uintptr {
	ret, _, _ := procGetForegroundWindow.Call()
	return ret
}

// 获取窗口标题长度
func getWindowTextLength(hwnd uintptr) int {
	ret, _, _ := procGetWindowTextLengthW.Call(hwnd)
	return int(ret)
}

// 获取窗口标题
func getWindowText(hwnd uintptr, str *uint16, maxCount int) int {
	ret, _, _ := procGetWindowTextW.Call(
		hwnd,
		uintptr(unsafe.Pointer(str)),
		uintptr(maxCount))
	return int(ret)
}

// 获取窗口所属进程ID
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

	// 获取窗口标题
	length := getWindowTextLength(hwnd) + 1
	buf := make([]uint16, length)
	getWindowText(hwnd, &buf[0], length)
	windowTitle := syscall.UTF16ToString(buf)

	// 获取进程ID
	var pid uint32
	getWindowThreadProcessId(hwnd, &pid)

	// 打开进程
	handle, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, pid)
	if err != nil {
		return windowTitle
	}
	defer syscall.CloseHandle(handle)

	// 获取进程可执行文件路径
	var exePath [syscall.MAX_PATH]uint16
	err = getProcessImageFileName(handle, &exePath[0], syscall.MAX_PATH)
	if err != nil {
		return windowTitle
	}

	// 从路径中提取文件名
	exeName := syscall.UTF16ToString(exePath[:])
	for i := len(exeName) - 1; i >= 0; i-- {
		if exeName[i] == '\\' {
			exeName = exeName[i+1:]
			break
		}
	}

	// 如果能获取到可执行文件名，返回它，否则返回窗口标题
	if exeName != "" {
		// 去掉扩展名
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

func monitorActiveApplication() {
	for {
		appName := getForegroundWindowInfo()
		currentStatus.UpdateStatus(appName)
		time.Sleep(time.Duration(config.RefreshInterval) * time.Millisecond)
	}
}

func main() {
	// 启动应用程序监控
	go monitorActiveApplication()

	// 设置路由
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/api/status", handleStatusAPI)

	// 启动服务器
	port := config.Port
	fmt.Printf("服务器启动在 http://localhost:%d\n", port)

	// 自动打开浏览器
	go func() {
		time.Sleep(1 * time.Second)
		openBrowser(fmt.Sprintf("http://localhost:%d", port))
	}()

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	default:
		err = fmt.Errorf("不支持的操作系统")
	}
	if err != nil {
		log.Printf("无法打开浏览器: %v", err)
	}
}

func handleStatusAPI(w http.ResponseWriter, r *http.Request) {
	username, app, timestamp := currentStatus.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"username":"%s","app":"%s","timestamp":"%s"}`,
		username, app, timestamp)
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	username, app, _ := currentStatus.GetStatus()

	tmpl := `	
<!DOCTYPE html>
<html>
<head>
    <title>What's {{.Username}} doing now?</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background-color: #f5f5f5;
        }
        .status-container {
            text-align: center;
        }
        .status-text {
            color: #ff6b6b;
            margin-bottom: 10px;
        } 
        .app-badge {
            display: inline-block;
            background-color: #ffeded;
            color: #ff6b6b;
            padding: 8px 15px;
            border-radius: 20px;
        }
        .app-icon {
            background-color: #fff;
            border-radius: 5px;
            padding: 3px;
            margin-right: 5px;
        }
    </style>
    <script>
        function updateStatus() {
            fetch('/api/status')
                .then(response => response.json())
                .then(data => {
                    document.getElementById('username').textContent = data.username;
                    document.getElementById('app').textContent = data.app;
                });
        }
        setInterval(updateStatus, {{.RefreshInterval}});
    </script>
</head>
<body>
    <div class="status-container">
        <p class="status-text"><span id="username">{{.Username}}</span> is using</p>
        <div class="app-badge">
            <span class="app-icon">⬜</span>
            <span id="app">{{.App}}</span>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("home").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Username        string
		App             string
		RefreshInterval int
	}{
		Username:        username,
		App:             app,
		RefreshInterval: config.RefreshInterval,
	}

	err = t.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
