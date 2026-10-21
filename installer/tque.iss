#ifndef AppVersion
  #error AppVersion is required
#endif
#ifndef StageDir
  #error StageDir is required
#endif
#ifndef OutputPath
  #error OutputPath is required
#endif
#ifndef TargetArch
  #define TargetArch "amd64"
#endif

[Setup]
AppId={{48DFF4F0-ECCF-4CFD-A61D-937F85F1C946}
AppName=tque
AppVersion={#AppVersion}
DefaultDirName={localappdata}\Programs\tque
DisableDirPage=yes
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
MinVersion=10.0.22000
#if TargetArch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif
OutputDir={#OutputPath}
OutputBaseFilename=tque-{#AppVersion}-windows-{#TargetArch}-setup
Compression=lzma2
SolidCompression=yes
ChangesEnvironment=yes
UninstallDisplayIcon={app}\tque.exe
LicenseFile={#StageDir}\LICENSE
#ifdef EnableSigning
SignTool=tque_sign
SignedUninstaller=yes
SignToolRunMinimized=yes
#endif

[Files]
Source: "{#StageDir}\tque.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\README.ja.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\docs\*"; DestDir: "{app}\docs"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#StageDir}\CHANGELOG.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\third-party-licenses\*"; DestDir: "{app}\third-party-licenses"; Flags: ignoreversion recursesubdirs createallsubdirs

[Code]
const
  OwnershipKey = 'Software\tque\Installer';

function NormalizeEntry(Value: String): String;
var
  StartAt, EndAt: Integer;
  Name, Expanded, Tail: String;
begin
  Value := RemoveQuotes(Trim(Value));
  Tail := Value;
  Value := '';
  while Pos('%', Tail) > 0 do begin
    StartAt := Pos('%', Tail);
    Value := Value + Copy(Tail, 1, StartAt - 1);
    Delete(Tail, 1, StartAt);
    EndAt := Pos('%', Tail);
    if EndAt = 0 then begin
      Value := Value + '%' + Tail;
      Tail := '';
      Break;
    end;
    Name := Copy(Tail, 1, EndAt - 1);
    Expanded := GetEnv(Name);
    if Expanded = '' then Value := Value + '%' + Name + '%'
    else Value := Value + Expanded;
    Delete(Tail, 1, EndAt);
  end;
  Value := Value + Tail;
  StringChangeEx(Value, '/', '\', True);
  Result := Lowercase(RemoveBackslashUnlessRoot(Value));
end;

function FindEntry(const Value, Target: String; var First, Count: Integer): Boolean;
var
  StartAt, EndAt: Integer;
begin
  Result := False;
  StartAt := 1;
  while StartAt <= Length(Value) do begin
    EndAt := StartAt;
    while (EndAt <= Length(Value)) and (Value[EndAt] <> ';') do EndAt := EndAt + 1;
    if NormalizeEntry(Copy(Value, StartAt, EndAt - StartAt)) = NormalizeEntry(Target) then begin
      First := StartAt;
      Count := EndAt - StartAt;
      if EndAt <= Length(Value) then Count := Count + 1
      else if StartAt > 1 then begin First := First - 1; Count := Count + 1; end;
      Result := True;
      Exit;
    end;
    StartAt := EndAt + 1;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Value, Target: String;
  First, Count: Integer;
begin
  if CurStep <> ssPostInstall then Exit;
  Target := ExpandConstant('{app}');
  Value := '';
  if RegValueExists(HKCU, 'Environment', 'Path') and
     not RegQueryStringValue(HKCU, 'Environment', 'Path', Value) then
    RaiseException('Cannot read user PATH.');
  { Existing ownership is deliberately left intact during an upgrade. }
  if not FindEntry(Value, Target, First, Count) then begin
    if (Value <> '') and (Copy(Value, Length(Value), 1) <> ';') then Value := Value + ';';
    Value := Value + Target;
    if not RegWriteExpandStringValue(HKCU, 'Environment', 'Path', Value) then
      RaiseException('Cannot update user PATH.');
    if not RegWriteStringValue(HKCU, OwnershipKey, 'AddedPath', Target) then
      RaiseException('Cannot record PATH ownership.');
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Value, Target: String;
  First, Count: Integer;
begin
  if CurUninstallStep <> usPostUninstall then Exit;
  if not RegQueryStringValue(HKCU, OwnershipKey, 'AddedPath', Target) then Exit;
  if RegValueExists(HKCU, 'Environment', 'Path') then begin
    if not RegQueryStringValue(HKCU, 'Environment', 'Path', Value) then
      RaiseException('Cannot read user PATH; ownership record retained.');
    if FindEntry(Value, Target, First, Count) then begin
      Delete(Value, First, Count);
      if not RegWriteExpandStringValue(HKCU, 'Environment', 'Path', Value) then
        RaiseException('Cannot update user PATH; ownership record retained.');
    end;
  end;
  RegDeleteValue(HKCU, OwnershipKey, 'AddedPath');
  RegDeleteKeyIfEmpty(HKCU, OwnershipKey);
end;
