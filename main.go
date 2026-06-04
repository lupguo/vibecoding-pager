package main

import (
	"embed"
	"log/slog"
	"os"

	infralog "github.com/lupguo/vibecoding-pager/internal/infra/log"
	"github.com/lupguo/vibecoding-pager/internal/wails"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	infralog.Init(slog.LevelInfo)

	app := wails.NewPagerApp(assets)
	if err := app.Run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
