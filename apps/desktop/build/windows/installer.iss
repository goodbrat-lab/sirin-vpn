; Inno Setup script для Sirin VPN Desktop
; Компиляция: ISCC.exe installer.iss

#define MyAppName "Sirin VPN"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Sirin Project"
#define MyAppURL "https://github.com/goodbrat-lab/sirin-vpn"
#define MyAppExeName "sirin.exe"

[Setup]
; App ID — уникальный GUID, НЕ менять между версиями (для обновления)
AppId={{8F3A2B1C-4D5E-6F7A-8B9C-0D1E2F3A4B5C}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\Sirin
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
; UAC: requireAdministrator (нужно для route add)
PrivilegesRequired=admin
OutputDir=..\..\dist
OutputBaseFilename=Sirin-VPN-{#MyAppVersion}-Setup
SetupIconFile=icon.ico
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; Минимальная версия Windows: 10
MinVersion=10.0
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64

[Languages]
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "autostart"; Description: "Запускать при входе в Windows"; GroupDescription: "Автозапуск:"; Flags: unchecked

[Files]
; Основные файлы (из bin/)
Source: "..\..\bin\sirin.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\bin\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
; Иконка
Source: "icon.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Registry]
; Автозапуск (если выбран)
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; \
    ValueType: string; ValueName: "SirinVPN"; \
    ValueData: """{app}\{#MyAppExeName}"""; \
    Flags: uninsdeletevalue; Tasks: autostart

[Run]
; Запустить после установки
Filename: "{app}\{#MyAppExeName}"; \
    Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; \
    Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Удалить конфиг пользователя при деинсталляции (опционально)
; Type: filesandordirs; Name: "{userappdata}\Sirin"
Type: filesandordirs; Name: "{app}\logs"

[Code]
// Проверка WebView2 (опционально, можно пропустить — Windows 10/11 имеет)
function InitializeSetup(): Boolean;
var
  WebView2Version: String;
begin
  Result := True;
  // WebView2 уже установлен на Windows 10/11 по умолчанию.
  // Если нет — можно добавить проверку и запуск установщика WebView2.
  // Пока просто пропускаем.
end;
