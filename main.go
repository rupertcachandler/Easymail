package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"

	"easymail/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

// Window icons etc. in v3 come from build/config.yml + the Taskfile asset
// pipeline; this PNG stays for packaging use.
//go:embed frontend/public/boi-icon.png
var appIcon []byte

func main() {
	easymail, err := app.New()
	if err != nil {
		println("Error:", err.Error())
		return
	}
	defer easymail.Shutdown()

	wailsApp := application.New(application.Options{
		Name:        "BOI",
		Description: "Easymail — BOI",
		Services: []application.Service{
			application.NewService(easymail),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	w := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "BOI",
		Width:            1200,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Hand the window to the App so SetBadge can update its title.
	easymail.SetWindow(w)

	if err := wailsApp.Run(); err != nil {
		println("Error:", err.Error())
	}
}
