//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// RegisterURIScheme регистрирует пользовательскую схему протокола sirin:// в HKCU
// (не требует прав администратора), чтобы ссылки sirin://key?uuid=... из Telegram
// автоматически открывали Sirin Desktop.
func RegisterURIScheme() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}

	runReg := func(args ...string) error {
		cmd := exec.Command("reg", args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Run()
	}

	if err := runReg("add", `HKCU\Software\Classes\sirin`, "/ve", "/d", "URL:Sirin VPN Protocol", "/f"); err != nil {
		return err
	}
	if err := runReg("add", `HKCU\Software\Classes\sirin`, "/v", "URL Protocol", "/t", "REG_SZ", "/d", "", "/f"); err != nil {
		return err
	}
	cmdVal := fmt.Sprintf(`"%s" "%%1"`, exe)
	return runReg("add", `HKCU\Software\Classes\sirin\shell\open\command`, "/ve", "/d", cmdVal, "/f")
}
