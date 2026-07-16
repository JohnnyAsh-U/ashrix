; installer.iss
; Compile with: iscc ashrix-connector.iss

#define AppName "Ashrix Connector"
#define AppVersion "1.0.0"
#define AppPublisher "Ashrix"
#define AppURL "https://ashrix.io"
#define AppExeName "ashrix-connector.exe"
#define TrayExeName "ashrix-tray.exe"
#define ServiceName "AshrixConnector"

[Setup]
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
DefaultDirName={autopf}\AshrixConnector
DefaultGroupName=Ashrix
OutputDir=dist
OutputBaseFilename=AshrixConnectorSetup
SetupIconFile=assets\ashrix.ico
UninstallDisplayIcon={app}\ashrix-connnector.exe
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
; Minimum Windows 10
MinVersion=10.0.17763

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"


[Tasks]
Name: "desktopicon"; \
  Description: "Start Ashrix Connector when Windows starts"; \
  GroupDescription: "Additional options:"; \
  Flags: unchecked
Name: "startup"; Description: "Start with Windows"; Flags: unchecked

[Files]
; Main binaries
Source: "{#AppExeName}";   DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{group}\{#AppName} (Tray)"; Filename: "{app}\{#AppExeName}"; Parameters: "tray"
Name: "{group}\Uninstall {#AppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon


[Run]
; Install service
Filename: "{app}\{#AppExeName}"; Parameters: "install"; Flags: runhidden

; Register with token if provided
Filename: "{app}\{#AppExeName}"; Parameters: "start --token ""{code:GetToken}"""; Flags: runhidden; Check: ShouldRegister

; If no token, just start with stored credential (if exists)
Filename: "{app}\{#AppExeName}"; Parameters: "start"; Flags: runhidden; Check: ShouldStartWithoutToken

; Launch tray
Filename: "{app}\{#AppExeName}"; Parameters: "tray"; Description: "Launch {#AppName}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\{#AppExeName}"; Parameters: "stop"; Flags: runhidden; RunOnceId: "StopService"
Filename: "{app}\{#AppExeName}"; Parameters: "uninstall"; Flags: runhidden; RunOnceId: "UninstallService"


[Code]
var
  TokenPage: TInputQueryWizardPage;

procedure InitializeWizard;
begin
  TokenPage := CreateInputQueryPage(wpWelcome,
    'Registration Token',
    'Enter your ephemeral token from the Control Plane',
    'This token is used to register your connector with the Ashrix Control Plane.' + #13#10 + 
    'The token is EPHEMERAL and will NOT be stored.' + #13#10 + #13#10 +
    'You need a new token each time you want to register.' + #13#10 + #13#10 +
    'If you don''t have a token, get one from: https://app.ashrix.io/tokens' + #13#10 + #13#10 +
    'You can skip this step and register later via CLI or tray menu.');

  TokenPage.Add('Ephemeral Token (optional):', False);
  TokenPage.Values[0] := '';
end;

function GetToken(Param: String): String;
begin
  Result := TokenPage.Values[0];
end;

function ShouldRegister: Boolean;
begin
  Result := (TokenPage.Values[0] <> '');
end;

function ShouldStartWithoutToken: Boolean;
begin
  Result := (TokenPage.Values[0] = '');
end;

function ShouldDeleteCredentials(): Boolean;
begin
  if MsgBox('Do you want to remove your stored credentials?' + #13#10 + #13#10 +
            'Select Yes to clear all credentials.' + #13#10 +
            'Select No to keep them (recommended if you plan to reinstall).',
            mbConfirmation, MB_YESNO) = idYes then
  begin
    Result := True;
  end
  else
  begin
    Result := False;
  end;
end;


[UninstallRun]
Filename: "{app}\{#AppExeName}"; Parameters: "clear-credential"; Flags: runhidden; Check: ShouldDeleteCredentials;

