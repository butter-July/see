package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

type Config struct {
	Username        string
	Port            int
	RefreshInterval int
}

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
func monitorApplication() {
	for {
		appName := getForegroundWindowInfo()
		currentStatus.UpdateStatus(appName)
		time.Sleep(time.Duration(config.RefreshInterval) * time.Millisecond)
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		// Linux服务器通常没有GUI，不尝试打开浏览器
		return
	}
	if err != nil {
		log.Printf("Failed to open browser: %v", err)
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

	tmpl := `<!DOCTYPE html>
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
</html>`

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

func main() {
	// 启动应用监控
	go monitorApplication()

	// 设置路由
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/api/status", handleStatusAPI)

	// 启动服务器
	port := config.Port
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Server starting on http://0.0.0.0%s\n", serverAddr)

	// 仅Windows平台尝试打开浏览器
	if runtime.GOOS == "windows" {
		go func() {
			time.Sleep(1 * time.Second)
			openBrowser(fmt.Sprintf("http://localhost%s", serverAddr))
		}()
	}

	// 启动HTTP服务
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}
