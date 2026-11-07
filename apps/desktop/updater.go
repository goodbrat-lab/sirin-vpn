package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const DesktopVersion = "1.6.0"

// UpdateInfo содержит информацию о доступном обновлении
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Tag            string `json:"tag"`
	DownloadURL    string `json:"download_url"`
	AssetName      string `json:"asset_name"`
	AssetSize      int64  `json:"asset_size"`
	ReleaseNotes   string `json:"release_notes"`
	PublishedAt    string `json:"published_at"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// isNewerVersion сравнивает две семантические версии (например, "1.6.1" > "1.6.0")
func isNewerVersion(remote, local string) bool {
	clean := func(v string) []int {
		v = strings.TrimSpace(v)
		v = strings.TrimPrefix(v, "v")
		v = strings.TrimPrefix(v, "desktop-v")
		v = strings.TrimPrefix(v, "router-v")
		if idx := strings.Index(v, "-"); idx != -1 {
			v = v[:idx]
		}
		parts := strings.Split(v, ".")
		res := make([]int, len(parts))
		for i, p := range parts {
			val, _ := strconv.Atoi(p)
			res[i] = val
		}
		return res
	}

	r := clean(remote)
	l := clean(local)

	maxLen := len(r)
	if len(l) > maxLen {
		maxLen = len(l)
	}

	for i := 0; i < maxLen; i++ {
		rPart := 0
		if i < len(r) {
			rPart = r[i]
		}
		lPart := 0
		if i < len(l) {
			lPart = l[i]
		}
		if rPart > lPart {
			return true
		}
		if rPart < lPart {
			return false
		}
	}
	return false
}

// CheckDesktopUpdate обращается к GitHub Releases API для проверки обновлений
func CheckDesktopUpdate(currentVer string) (*UpdateInfo, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/goodbrat-lab/sirin-vpn/releases", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Sirin-Desktop-Updater")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	var releases []ghRelease
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		_ = json.NewDecoder(resp.Body).Decode(&releases)
	} else {
		if resp != nil {
			resp.Body.Close()
		}
		// Fallback to /releases/latest
		reqLatest, _ := http.NewRequest("GET", "https://api.github.com/repos/goodbrat-lab/sirin-vpn/releases/latest", nil)
		if reqLatest != nil {
			reqLatest.Header.Set("User-Agent", "Sirin-Desktop-Updater")
			reqLatest.Header.Set("Accept", "application/vnd.github+json")
			if respLatest, errLatest := client.Do(reqLatest); errLatest == nil && respLatest.StatusCode == http.StatusOK {
				defer respLatest.Body.Close()
				var single ghRelease
				if json.NewDecoder(respLatest.Body).Decode(&single) == nil {
					releases = append(releases, single)
				}
			} else if respLatest != nil {
				respLatest.Body.Close()
			}
		}
	}

	if len(releases) == 0 {
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: currentVer,
			LatestVersion:  currentVer,
		}, nil
	}

	for _, rel := range releases {
		if rel.Draft {
			continue
		}

		tag := strings.TrimSpace(rel.TagName)
		verClean := strings.TrimPrefix(tag, "v")
		verClean = strings.TrimPrefix(verClean, "desktop-v")
		if idx := strings.Index(verClean, "-"); idx != -1 {
			verClean = verClean[:idx]
		}

		if !isNewerVersion(verClean, currentVer) {
			continue
		}

		// Find Windows .exe asset
		for _, asset := range rel.Assets {
			lowerName := strings.ToLower(asset.Name)
			if strings.HasSuffix(lowerName, ".exe") {
				notes := strings.TrimSpace(rel.Body)
				if len(notes) > 500 {
					notes = notes[:497] + "..."
				}

				return &UpdateInfo{
					Available:      true,
					CurrentVersion: currentVer,
					LatestVersion:  verClean,
					Tag:            tag,
					DownloadURL:    asset.BrowserDownloadURL,
					AssetName:      asset.Name,
					AssetSize:      asset.Size,
					ReleaseNotes:   notes,
					PublishedAt:    rel.PublishedAt.Format("02.01.2006"),
				}, nil
			}
		}
	}

	// No newer update
	latestVer := currentVer
	if len(releases) > 0 {
		t := releases[0].TagName
		t = strings.TrimPrefix(t, "v")
		t = strings.TrimPrefix(t, "desktop-v")
		if idx := strings.Index(t, "-"); idx != -1 {
			t = t[:idx]
		}
		if t != "" {
			latestVer = t
		}
	}

	return &UpdateInfo{
		Available:      false,
		CurrentVersion: currentVer,
		LatestVersion:  latestVer,
	}, nil
}

// ApplyDesktopUpdate скачивает бинарник и запускает безопасную замену текущего исполняемого файла
func ApplyDesktopUpdate(downloadURL string, emitLog func(string, string)) error {
	if downloadURL == "" {
		return fmt.Errorf("пустой URL для загрузки обновления")
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("определение пути исполняемого файла: %w", err)
	}

	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	updatesDir := filepath.Join(appData, "Sirin", "updates")
	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		return fmt.Errorf("создание каталога обновлений: %w", err)
	}

	tempExe := filepath.Join(updatesDir, "sirin_update.exe")
	_ = os.Remove(tempExe)

	emitLog(fmt.Sprintf("Загрузка обновления из %s...", downloadURL), "info")

	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("User-Agent", "Sirin-Desktop-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка загрузки: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("сервер вернул статус %d", resp.StatusCode)
	}

	out, err := os.Create(tempExe)
	if err != nil {
		return fmt.Errorf("создание целевого файла: %w", err)
	}

	written, err := io.Copy(out, resp.Body)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tempExe)
		return fmt.Errorf("запись файла обновления: %w", err)
	}

	if written < 500*1024 {
		_ = os.Remove(tempExe)
		return fmt.Errorf("файл обновления слишком мал (%d байт), загрузка повреждена", written)
	}

	// Проверяем PE заголовок ('M', 'Z')
	header := make([]byte, 2)
	f, err := os.Open(tempExe)
	if err == nil {
		_, _ = f.Read(header)
		_ = f.Close()
		if header[0] != 'M' || header[1] != 'Z' {
			_ = os.Remove(tempExe)
			return fmt.Errorf("загруженный файл не является валидным исполняемым файлом Windows")
		}
	}

	emitLog("Файл обновления проверен. Подготовка к перезапуску...", "info")

	// Создаем batch-скрипт обновления
	scriptPath := filepath.Join(updatesDir, "apply_update.cmd")
	scriptContent := `@echo off
chcp 65001 >nul
setlocal
set PID=%1
set SRC=%~2
set DST=%~3

timeout /t 1 /nobreak >nul
:wait_loop
tasklist /fi "PID eq %PID%" 2>nul | findstr /i "%PID%" >nul
if not errorlevel 1 (
    timeout /t 1 /nobreak >nul
    goto wait_loop
)

copy /y "%SRC%" "%DST%" >nul
if errorlevel 1 (
    timeout /t 2 /nobreak >nul
    copy /y "%SRC%" "%DST%" >nul
)

del /f /q "%SRC%" >nul 2>&1
start "" "%DST%"
(goto) 2>nul & del "%~f0"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("создание скрипта применения обновления: %w", err)
	}

	pid := os.Getpid()
	cmd := exec.Command("cmd.exe", "/c", scriptPath, strconv.Itoa(pid), tempExe, currentExe)
	// CREATE_NO_WINDOW (0x08000000) | CREATE_NEW_PROCESS_GROUP (0x00000200)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | 0x00000200,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск процесса обновления: %w", err)
	}

	emitLog("Обновление установлено. Приложение перезапускается через 2 секунды...", "info")

	go func() {
		time.Sleep(1500 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}
