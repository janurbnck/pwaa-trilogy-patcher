package main

import (
	"changeme/backend"
	"embed"
	"log"
	"path/filepath"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

type OperationResult struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type PatcherService struct{}

func init() {
	if backend.Config == nil || len(backend.Config.PatcherDisplayText) == 0 {
		for _, p := range []string{"config.json", filepath.Join("..", "Data", "config.json"), filepath.Join("..", "config.json")} {
			if cfg, err := backend.LoadConfig(p); err == nil && len(cfg.PatcherDisplayText) > 0 {
				backend.Config = cfg
				break
			}
		}
	}
}

func (p *PatcherService) GetAvailableVersions() []string {
	if backend.Config != nil {
		return backend.Config.AvailableVersions
	}
	return []string{"0.1.1b", "0.1.1a", "0.1.1", "orig"}
}

func (p *PatcherService) SelectGameFolder() (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Vyberte kořenovou složku hry").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
}

func (p *PatcherService) DetectVersion(gameDir string) string {
	if record, err := backend.LoadInstallRecord(filepath.Join(gameDir, "translation_status.json")); err == nil && record.Version != "" {
		return record.Version
	}
	if exe, err := backend.DetectExe(gameDir); err == nil && exe != "" {
		return "orig"
	}
	return "error"
}

func (p *PatcherService) GetDisplayText() map[string]string {
	if backend.Config != nil {
		return backend.Config.PatcherDisplayText
	}
	return map[string]string{}
}

func (p *PatcherService) Patch(gameDir string, newVersion string) OperationResult {
	ok, code := backend.PatchAllFiles(gameDir, newVersion)
	msg := "Patch nebyl nainstalován.\nPodrobnější informace o chybě najdete v logu."
	if backend.Config != nil {
		if m, exists := backend.Config.PatchErrorMap[strconv.Itoa(code)]; exists {
			msg = m
		}
	}
	return OperationResult{Success: ok, Code: code, Message: msg}
}

func (p *PatcherService) Unpatch(gameDir string) OperationResult {
	ok, code := backend.UnpatchAllFiles(gameDir)
	msg := "Data nebyla obnovena.\nPodrobnější informace o chybě najdete v logu."
	if backend.Config != nil {
		if m, exists := backend.Config.UnpatchErrorMap[strconv.Itoa(code)]; exists {
			msg = m
		}
	}
	return OperationResult{Success: ok, Code: code, Message: msg}
}

func main() {
	if backend.LogFile != nil {
		defer backend.LogFile.Close()
	}

	app := application.New(application.Options{
		Name:     "pwaa-trilogy-patcher",
		Services: []application.Service{application.NewService(&PatcherService{})},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "PWAAT Patcher",
		Width:            340,
		Height:           320,
		InitialPosition:  application.WindowCentered,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(24, 24, 27),
		Windows:          application.WindowsWindow{Theme: application.Dark},
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
