package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var Config, loadConfErr = LoadConfig(filepath.Join("Data", "config.json"))
var LogFile, setupLogErr = SetupLogging()

func DoesXdeltaFileExist() bool {
	if !FilepathExists(Xdelta) {
		slog.Error("xdelta.exe not found.", slog.String("path", Xdelta))
		return false
	}
	return true
}

func DetectExe(game_dir string) (string, error) {
	exe, err := filepath.Glob(filepath.Join(game_dir, "PWAAT.exe"))
	if err != nil {
		slog.Error("Game .exe file not found.")
		return "", err
	}
	if len(exe) > 0 {
		slog.Info("Game .exe file found.", slog.String("path", exe[0]))
		return exe[0], nil
	}
	slog.Error("Game .exe file not found.")
	return "", os.ErrNotExist
}

func SaveInstallRecord(gameDir string, currentVersion string) error {
	record := InstallRecord{
		Version:   currentVersion,
		Installed: true,
		Backup:    filepath.Join(gameDir, "Backup"),
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(gameDir, "translation_status.json")

	return os.WriteFile(filePath, data, 0644)
}

func DeleteInstallRecord(gameDir string) error {
	return os.Remove(filepath.Join(gameDir, "translation_status.json"))
}

func GetOldVersionPath(version string) (string, error) {
	existingVSwitcher := Config.ExistingVSwitcher
	if path, ok := existingVSwitcher[version]; ok {
		slog.Info("Old version path found.", slog.String("path", path))
		return path, nil
	}

	slog.Error("Old version path not found.", slog.String("version", version))
	return "", os.ErrNotExist
}

func GetNewVersionPath(version string) (string, error) {
	newVSwitcher := Config.NewVSwitcher
	if path, ok := newVSwitcher[version]; ok {
		slog.Info("New version path found.", slog.String("path", path))
		return path, nil
	}

	slog.Error("New version path not found.", slog.String("version", version))
	return "", os.ErrNotExist
}

func GetPatchDirectory(old_v_path string, new_v_path string) (string, error) {
	if old_v_path == "" || new_v_path == "" {
		slog.Error("Old or new version path is empty.", slog.String("old_version_path", old_v_path), slog.String("new_version_path", new_v_path))
		return "", os.ErrNotExist
	}

	slog.Info("Patch directory determined.", slog.String("path", filepath.Join("Patches", filepath.Base(old_v_path), filepath.Base(new_v_path))))
	return filepath.Join("Patches", filepath.Base(old_v_path), filepath.Base(new_v_path)), nil
}

func FindMatchingFile(patchFile string, gameDir string, patchDir string) (string, error) {
	relativePath, err := filepath.Rel(patchDir, patchFile)
	if err != nil {
		slog.Error("Failed to resolve relative path", slog.String("patchDir", patchDir), slog.String("patchFile", patchFile))
		return "", err
	}

	patchBase := strings.TrimSuffix(filepath.Base(relativePath), ".xdelta")
	relativeFolder := filepath.Dir(relativePath)
	targetFolder := filepath.Join(gameDir, relativeFolder)

	if !FilepathExists(targetFolder) {
		slog.Error("Target folder does not exist.", slog.String("path", targetFolder))
		return "", os.ErrNotExist
	}

	entries, err := os.ReadDir(targetFolder)
	if err != nil {
		slog.Error("Failed to read target folder.", slog.String("path", targetFolder))
		return "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			entryName := entry.Name()
			entryBase := filepath.Base(entryName)

			if entryBase == patchBase {
				return filepath.Join(targetFolder, entryName), nil
			}
		}
	}

	slog.Error("Matching file not found.", slog.String("relativePath", relativePath))
	return "", os.ErrNotExist
}

func CreateTempDirectory() (string, error) {
	tmpDirName, err := os.MkdirTemp("", "pwaa-trilogy-patcher")
	if err != nil {
		slog.Error("Failed to create temporary directory.", slog.String("error", err.Error()))
		return "", err
	}

	slog.Info("Temporary directory created.", slog.String("path", tmpDirName))
	return tmpDirName, nil
}

func DeleteTempDirectory(tmpDir string) error {
	if FilepathExists(tmpDir) {
		slog.Info("Temporary directory deleted.", slog.String("path", tmpDir))
		return os.RemoveAll(tmpDir)
	}

	slog.Error("Temporary directory does not exist.", slog.String("path", tmpDir))
	return nil
}

func DeleteBackupDirectory(backupDir string) error {
	if FilepathExists(backupDir) {
		slog.Info("Backup directory deleted.", slog.String("path", backupDir))
		return os.RemoveAll(backupDir)
	}

	slog.Error("Backup directory does not exist.", slog.String("path", backupDir))
	return nil
}

func CreateBackupDirectory(gameDir string) (string, error) {
	backupDir := filepath.Join(gameDir, "Backup")

	err := os.MkdirAll(backupDir, 0755)
	if err != nil {
		slog.Error("Failed to create backup directory.", slog.String("path", backupDir))
		return "", err
	}

	slog.Info("Backup directory created.", slog.String("path", backupDir))
	return backupDir, nil
}

func GetPatchFiles(newVersion string, data map[string][]string, lookup map[string]string) []string {
	patchFiles := GetBackupFiles(newVersion, data, lookup)

	for i, _ := range patchFiles {
		patchFiles[i] = patchFiles[i] + ".xdelta"
	}

	return patchFiles
}

func GetBackupFiles(newVersion string, data map[string][]string, lookup map[string]string) []string {
	if newVersion == "" {
		return []string{}
	}

	slog.Info("New backup files found.")
	files := append([]string{}, data[newVersion]...)
	return append(files, GetBackupFiles(lookup[newVersion], data, lookup)...)
}

func VerifyPatchFiles(foundPatchFiles []string, newVersion string) bool {
	expectedPatchFiles := GetPatchFiles(newVersion, Config.ExpectedFiles, Config.VersionLookup)
	foundNames := make([]string, len(foundPatchFiles))
	for i, f := range foundPatchFiles {
		foundNames[i] = filepath.Base(f)
	}

	if !DoListsMatch(foundNames, expectedPatchFiles) {
		slog.Error("Patch files do not match expected files.", slog.String("found", fmt.Sprintf("%v", foundNames)), slog.String("expected", fmt.Sprintf("%v", expectedPatchFiles)))
		return false
	}

	return true
}

func VerifyBackupFiles(foundBackupFiles []string, newVersion string) bool {
	expectedBackupFiles := GetBackupFiles(newVersion, Config.ExpectedFiles, Config.VersionLookup)
	foundNames := make([]string, len(foundBackupFiles))
	for i, f := range foundBackupFiles {
		foundNames[i] = filepath.Base(f)
	}

	if !DoListsMatch(foundNames, expectedBackupFiles) {
		slog.Error("Backup files do not match expected files.", slog.String("found", fmt.Sprintf("%v", foundNames)), slog.String("expected", fmt.Sprintf("%v", expectedBackupFiles)))
		return false
	}

	return true
}

func FindBackupFiles(backupDir string) ([]string, error) {
	var origFiles []string
	err := filepath.WalkDir(backupDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			fileName := d.Name()

			for _, ext := range Config.Extensions {
				matched, _ := filepath.Match(ext, fileName)
				if matched {
					origFiles = append(origFiles, path)
					break
				}
			}
		}
		return nil
	})

	if err != nil {
		slog.Error("Error while walking through backup directory.", slog.String("path", backupDir), slog.String("error", err.Error()))
		return nil, err
	}

	return origFiles, nil
}

func FindPatchFiles(patchDir string) ([]string, error) {
	var patchFiles []string
	err := filepath.WalkDir(patchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			patchFiles = append(patchFiles, path)
		}
		return nil
	})

	if err != nil {
		slog.Error("Error while walking through patch directory.", slog.String("path", patchDir), slog.String("error", err.Error()))
		return nil, err
	}

	return patchFiles, nil
}

func ApplyPatch(xdeltaFile, patchFile, origFile, outFile string) error {
	cmd := exec.Command(
		xdeltaFile,
		"-d",
		"-f",
		"-s",
		origFile,
		patchFile,
		outFile,
	)
	HideWindow(cmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		slog.Error("Error occurred while applying patch.", slog.String("error", stderr.String()))
		return err
	}

	slog.Info("Patch applied successfully.", slog.String("patchFile", patchFile), slog.String("origFile", origFile), slog.String("outFile", outFile))

	return nil
}

func UnpatchAllFiles(gameDir string) (bool, int) {
	exe, exeErr := DetectExe(gameDir)
	if exeErr != nil || exe == "" {
		slog.Error("Game .exe file not found. Cannot unpatch files.", slog.String("path", gameDir))
		return false, 404
	}

	backupDir := filepath.Join(gameDir, "Backup")
	if !FilepathExists(backupDir) {
		slog.Error("Backup directory not found. Cannot unpatch files.", slog.String("path", backupDir))
		return false, 403
	}

	slog.Info("Backup directory found. Proceeding to unpatch files.", slog.String("path", backupDir))
	checkFile, err := LoadInstallRecord(filepath.Join(gameDir, "translation_status.json"))
	if err != nil {
		slog.Error("Failed to load install record.", slog.String("error", err.Error()))
		return false, 401
	}

	backupFiles, err := FindBackupFiles(backupDir)
	if err != nil {
		slog.Error("Failed to find backup files.", slog.String("error", err.Error()))
		return false, 402
	}

	if !VerifyBackupFiles(backupFiles, checkFile.Version) {
		slog.Error("Backup files verification failed. Cannot unpatch files.", slog.String("version", checkFile.Version))
		return false, 402
	}

	for _, backupFile := range backupFiles {
		matchedFile, matchErr := FindMatchingFile(backupFile, gameDir, backupDir)
		if matchErr != nil {
			slog.Error("Failed to find matching file.", slog.String("backupFile", backupFile), slog.String("error", matchErr.Error()))
			return false, 405
		}

		copyErr := CopyFile(backupFile, matchedFile)
		if copyErr != nil {
			slog.Error("Failed to copy file.", slog.String("backupFile", backupFile), slog.String("matchedFile", matchedFile), slog.String("error", copyErr.Error()))
			return false, 406
		}
		slog.Info("File unpatched successfully.", slog.String("backupFile", backupFile), slog.String("matchedFile", matchedFile))
	}

	DeleteBackupDirectory(backupDir)
	DeleteInstallRecord(gameDir)
	slog.Info("All files unpatched successfully.")

	return true, 0
}

func PatchAllFiles(gameDir string, newVersion string) (bool, int) {
	slog.Info("Starting preparation for patching.", slog.String("gameDir", gameDir), slog.String("newVersion", newVersion))

	slog.Info("Searching for xdelta.exe.")
	if !DoesXdeltaFileExist() {
		slog.Error("xdelta.exe not found. Cannot proceed with patching.", slog.String("path", Xdelta))
		return false, 401
	}
	slog.Info("xdelta.exe found.", slog.String("path", Xdelta))

	slog.Info("Searching for game .exe file.")
	exe, exeErr := DetectExe(gameDir)
	if exeErr != nil || exe == "" {
		slog.Error("Game .exe file not found. Cannot proceed with patching.", slog.String("path", gameDir))
		return false, 404
	}
	slog.Info("Game .exe file found.", slog.String("path", exe))

	slog.Info("Searching for install record.")
	oldVersion := "orig"
	checkFile, err := LoadInstallRecord(filepath.Join(gameDir, "translation_status.json"))
	if err != nil {
		slog.Info("No install record found. Proceeding with patching.")
	} else if checkFile.Version == newVersion {
		slog.Info("This version is already installed. No patching needed.", slog.String("version", newVersion))
		return true, 101
	} else {
		oldVersion = checkFile.Version
		slog.Info("Install record found. Proceeding with patching.", slog.String("installedVersion", checkFile.Version))
	}

	slog.Info("Checking current version.")
	oldPath, oldPathErr := GetOldVersionPath(oldVersion)
	if oldPathErr != nil {
		slog.Error("Patching from this version is not supported.", slog.String("version", oldVersion))
		return false, 403
	}
	slog.Info("Patching from this version is supported.", slog.String("oldVersionPath", oldPath))

	slog.Info("Checking target version.")
	newPath, newPathErr := GetNewVersionPath(newVersion)
	if newPathErr != nil {
		slog.Error("Patching to this version is not supported.", slog.String("version", newVersion))
		return false, 404
	}
	slog.Info("Patching to this version is supported.", slog.String("newVersionPath", newPath))

	slog.Info("Searching for patch directory.")
	patchDir, patchDirErr := GetPatchDirectory(oldPath, newPath)
	if patchDirErr != nil {
		slog.Error("Patch directory not found.", slog.String("oldVersionPath", oldPath), slog.String("newVersionPath", newPath))
		return false, 405
	}
	slog.Info("Patch directory found.", slog.String("patchDir", patchDir))

	slog.Info("Searching for patch files.")
	patchFiles, patchFilesErr := FindPatchFiles(patchDir)
	if patchFilesErr != nil {
		slog.Error("Failed to find patch files.", slog.String("patchDir", patchDir), slog.String("error", patchFilesErr.Error()))
		return false, 405
	}
	slog.Info("Patch files found.")

	slog.Info("Verifying patch files.")
	if !VerifyPatchFiles(patchFiles, newVersion) {
		slog.Error("Failed to verify patch files.", slog.String("newVersion", newVersion))
		return false, 408
	}
	slog.Info("Patch files verified.")

	slog.Info("Creating temporary directory.")
	tempDir, tmpErr := CreateTempDirectory()
	if tmpErr != nil {
		slog.Error("Failed to create temporary directory.")
		return false, 412
	}
	defer DeleteTempDirectory(tempDir)

	slog.Info("Creating backup directory.")
	backupDir, bckpDirErr := CreateBackupDirectory(gameDir)
	if bckpDirErr != nil {
		slog.Error("Failed to create backup directory.")
		return false, 413
	}

	slog.Info("Preparation for patching completed successfully. Proceeding to patch files.")

	matchedFiles := make([]string, 0, len(patchFiles))
	tempFiles := make([]string, 0, len(patchFiles))

	for _, patchFile := range patchFiles {
		slog.Info("Finding matching file for patch.", slog.String("patchFile", patchFile))
		matchedFile, matchErr := FindMatchingFile(patchFile, gameDir, patchDir)
		if matchErr != nil {
			slog.Error("Failed to find matching file for patch.", slog.String("patchFile", patchFile), slog.String("error", matchErr.Error()))
			return false, 407
		}
		slog.Info("Matching file found for patch.", slog.String("patchFile", patchFile), slog.String("matchedFile", matchedFile))
		relativePath, relErr := filepath.Rel(gameDir, matchedFile)
		if relErr != nil {
			slog.Error("Failed to get relative path.", slog.String("matchedFile", matchedFile), slog.String("error", relErr.Error()))
			return false, 409
		}
		tempFile := filepath.Join(tempDir, relativePath)
		os.MkdirAll(filepath.Dir(tempFile), 0755)
		backupFile := filepath.Join(backupDir, relativePath)
		os.MkdirAll(filepath.Dir(backupFile), 0755)

		copyErr := CopyFile(matchedFile, backupFile)
		if copyErr != nil {
			slog.Error("Failed to backup file.", "error", copyErr)
			return false, 410
		}

		matchedFiles = append(matchedFiles, matchedFile)
		tempFiles = append(tempFiles, tempFile)

		slog.Info("File backed up successfully.", slog.String("matchedFile", matchedFile), slog.String("backupFile", backupFile))
	}

	j := 0
	for _, patchFile := range patchFiles {
		slog.Info("Applying patch.", slog.String("patchFile", patchFile))
		patchResult := ApplyPatch(Xdelta, patchFile, matchedFiles[j], tempFiles[j])
		if patchResult != nil {
			slog.Error("Failed to apply patch.", slog.String("patchFile", patchFile), slog.String("error", patchResult.Error()))
			return false, 501
		}
		j++
	}

	slog.Info("All patches applied successfully. Proceeding to copy patched files to game directory.")
	for _, patchFile := range patchFiles {
		matchedFile, matchErr := FindMatchingFile(patchFile, gameDir, patchDir)
		if matchErr != nil {
			slog.Error("Failed to find matching file for patch.", slog.String("patchFile", patchFile), slog.String("error", matchErr.Error()))
			return false, 407
		}
		relFile, relErr := filepath.Rel(gameDir, matchedFile)
		if relErr != nil {
			slog.Error("Failed to get relative path.", slog.String("matchedFile", matchedFile), slog.String("error", relErr.Error()))
			return false, 409
		}
		tempFile := filepath.Join(tempDir, relFile)
		copyErr := CopyFile(tempFile, matchedFile)
		if copyErr != nil {
			slog.Error("Failed to copy patched file to game directory.", slog.String("tempFile", tempFile), slog.String("matchedFile", matchedFile), slog.String("error", copyErr.Error()))
			return false, 411
		}
		slog.Info("Patched file copied to game directory successfully.", slog.String("tempFile", tempFile), slog.String("matchedFile", matchedFile))
	}
	slog.Info("All patched files copies successfully.")

	instlRecordErr := SaveInstallRecord(gameDir, newVersion)
	if instlRecordErr != nil {
		slog.Error("Failed to create check file.", "error", instlRecordErr)
		return false, 414
	}
	slog.Info("Check file created.")

	slog.Info("Patching process complete.")

	return true, 0
}
