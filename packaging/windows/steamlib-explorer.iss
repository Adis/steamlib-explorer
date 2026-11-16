; Inno Setup script for the Windows installer. The release workflow runs
;   iscc /DAppVersion=1.2.3 /DSourceExe=path\to\steamlib-explorer.exe steamlib-explorer.iss
; Installs for the current user only, so it needs no administrator rights.

#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
; Numeric part for the Windows version resource ("1.2.3" from "1.2.3-dev").
#define AppNumericVersion Copy(AppVersion, 1, Pos("-", AppVersion + "-") - 1)
#ifndef SourceExe
  #define SourceExe "..\..\backend\bin\steamlib-explorer.exe"
#endif

[Setup]
AppId={{6F1C3A52-8D47-4B8E-9E43-2C5B7A9D1E10}
AppName=Steamlib Explorer
AppVersion={#AppVersion}
AppPublisher=Steamlib Explorer
AppPublisherURL=https://github.com/Adis/steamlib-explorer
; Code signing checks these: same product name and version as the .exe inside.
VersionInfoVersion={#AppNumericVersion}
VersionInfoProductName=Steamlib Explorer
VersionInfoProductVersion={#AppNumericVersion}
VersionInfoProductTextVersion={#AppVersion}
VersionInfoDescription=Steamlib Explorer Setup
DefaultDirName={localappdata}\Programs\Steamlib Explorer
DefaultGroupName=Steamlib Explorer
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=Output
OutputBaseFilename=steamlib-explorer_{#AppVersion}_windows_setup
SetupIconFile=..\..\backend\cmd\steamlib-explorer\icon\icon.ico
UninstallDisplayIcon={app}\steamlib-explorer.exe
LicenseFile=..\..\LICENSE
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=force

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "steamlib-explorer.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\Steamlib Explorer"; Filename: "{app}\steamlib-explorer.exe"
Name: "{autodesktop}\Steamlib Explorer"; Filename: "{app}\steamlib-explorer.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\steamlib-explorer.exe"; Description: "{cm:LaunchProgram,Steamlib Explorer}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; The app keeps running in the tray; stop it so its file can be removed.
Filename: "{sys}\taskkill.exe"; Parameters: "/IM steamlib-explorer.exe /F"; Flags: runhidden; RunOnceId: "StopApp"

; Your library and settings in %AppData%\steamlib-explorer are kept on uninstall.
