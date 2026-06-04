## Wails v3 CLI 通过 Go install 安装，更新方式：

```shell

# 方法 1：直接安装最新 alpha
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

# 方法 2：安装与 go.mod 一致的版本 (alpha.96)
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96

安装后验证： wails3 version

如果 wails3 命令找不到，确保 $GOPATH/bin（通常是 ~/go/bin）在你的 $PATH 里：
export PATH="$PATH:$(go env GOPATH)/bin"

更新完后就可以在项目目录运行：
cd /path/to/pager
wails3 dev    # 开发模式（热重载）

# 或
wails3 build  # 构建 .app
```

## 
