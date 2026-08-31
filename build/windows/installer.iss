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
Name: "startup"; Description: "Start with Windows"; Flags: unchecked

[Files]
; Main binaries
Source: "{#AppExeName}";   DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{group}\Uninstall {#AppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon


[Run]

; Install service
Filename: "{app}\{#AppExeName}"; Parameters: "install"; Flags: runhidden;

[UninstallRun]
Filename: "{app}\{#AppExeName}"; Parameters: "stop"; Flags: runhidden; RunOnceId: "StopService"
Filename: "{app}\{#AppExeName}"; Parameters: "uninstall"; Flags: runhidden; RunOnceId: "UninstallService"
