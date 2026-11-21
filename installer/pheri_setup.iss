[Setup]
AppName=Pheri Database Manager
AppVersion=3.3.0
AppPublisher=Vicky Chhetri
AppPublisherURL=https://github.com/vickychhetri/pheri
DefaultDirName={autopf}\Pheri
DefaultGroupName=Pheri
OutputBaseFilename=pheri-setup-windows-amd64
Compression=lzma2/max
SolidCompression=yes
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog commandline
ChangesEnvironment=yes

[Files]
Source: "..\build\pheri-windows-amd64.exe"; DestDir: "{app}"; DestName: "pheri.exe"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Pheri"; Filename: "powershell.exe"; Parameters: "-NoExit -Command ""& '{app}\pheri.exe'"""
Name: "{group}\Uninstall Pheri"; Filename: "{uninstallexe}"

[Registry]
; Adds {app} to User PATH environment variable
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Check: NeedsAddPath(ExpandConstant('{app}'))

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath)
  then begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + UpperCase(Param) + ';', ';' + UpperCase(OrigPath) + ';') = 0;
end;