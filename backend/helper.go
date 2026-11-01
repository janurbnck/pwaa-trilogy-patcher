package backend

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type MasterConfig struct {
	AvailableVersions  []string            `json:"available_versions"`
	UnpatchErrorMap    map[string]string   `json:"unpatch_error_map"`
	PatchErrorMap      map[string]string   `json:"patch_error_map"`
	ExistingVSwitcher  map[string]string   `json:"existing_v_switcher"`
	NewVSwitcher       map[string]string   `json:"new_v_switcher"`
	ExpectedFiles      map[string][]string `json:"expected_files"`
	VersionLookup      map[string]string   `json:"version_lookup"`
	PatcherDisplayText map[string]string   `json:"patcher_display_text"`
	Extensions         []string            `json:"extensions"`
}

type InstallRecord struct {
	Version   string `json:"installed_version"`
	Installed bool   `json:"installed"`
	Backup    string `json:"backup"`
}

func FilepathExists(path string) bool {
	_, err := os.Stat(path)

	if err == nil {
		return true
	}

	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	return false
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func GetAppDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	realPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		realPath = exePath
	}

	thisDir := filepath.Dir(realPath)

	return thisDir, nil
}

func LoadConfig(filePath string) (*MasterConfig, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config MasterConfig

	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func LoadInstallRecord(filePath string) (*InstallRecord, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var installRecord InstallRecord

	err = json.Unmarshal(bytes, &installRecord)
	if err != nil {
		return nil, err
	}

	return &installRecord, nil
}

func SetupLogging() (*os.File, error) {
	logFile, err := os.OpenFile("patcher_info.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	logger := slog.New(slog.NewTextHandler(logFile, opts))
	slog.SetDefault(logger)

	return logFile, nil
}

func DoListsMatch(list1, list2 []string) bool {
	if len(list1) != len(list2) {
		return false
	}

	counts := make(map[string]int)

	for _, item := range list1 {
		counts[item]++
	}

	for _, item := range list2 {
		counts[item]--

		if counts[item] < 0 {
			return false
		}
	}

	return true
}
