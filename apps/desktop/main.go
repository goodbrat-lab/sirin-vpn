package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	appService := NewApp()

	// Регистрируем URI-схему sirin:// в HKCU (best-effort)
	_ = RegisterURIScheme()

	// Если запущены по deep-link (sirin://key?uuid=...), сразу импортируем ключ
	for _, arg := range os.Args[1:] {
		if uuid := ParseSirinDeepLink(arg); uuid != "" {
			if err := appService.SaveKey(uuid); err == nil {
				log.Printf("[deeplink] Ключ успешно импортирован из ссылки")
			}
			break
		}
	}

	app := application.New(application.Options{
		Name:        "Сирин",
		Description: "Клиент Sirin VPN",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Сирин",
		Width:  400,
		Height: 600,
		BackgroundColour: application.NewRGB(10, 14, 23),
		URL:              "/",
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
