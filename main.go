package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"easymail/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/public/boi-icon.png
var appIcon []byte

func main() {
	easymail, err := app.New()
	if err != nil {
		println("Error:", err.Error())
		return
	}
	defer easymail.Shutdown()

	err = wails.Run(&options.App{
		Title:         "BOI",
		Width:         1200,
		Height:        800,
		MinWidth:      900,
		MinHeight:     600,
		AssetServer:   &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     easymail.Startup,
		OnShutdown:    easymail.OnShutdown,
		OnBeforeClose: easymail.OnBeforeClose,
		Bind: []interface{}{
			easymail,
		},
		Linux: &linux.Options{
			Icon:                appIcon,
			WindowIsTranslucent: false,
			WebviewGpuPolicy:   linux.WebviewGpuPolicyAlways,
			ProgramName:         "easymail",
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}