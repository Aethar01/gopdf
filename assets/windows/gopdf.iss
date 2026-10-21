#define MyAppName "GoPDF"
#define MyAppExeName "gopdf.exe"
; As gopdf sets itself, so the taskbar groups its windows with these
; shortcuts and lists recent files in its jump list.
#define MyAppUserModelID "app.gopdf.viewer"
#define MyAppAssocName "PDF Document"
#define MyAppAssocExt ".pdf"
#define MyAppAssocKey StringChange(MyAppName, " ", "") + MyAppAssocExt
; The other formats gopdf opens are offered under Open With, by these.
#define MyAppDocumentKey StringChange(MyAppName, " ", "") + ".Document"
#define MyAppImageKey StringChange(MyAppName, " ", "") + ".Image"

[Setup]
AppId={{A0D76E2B-6B7D-4E84-A93A-8C4F8C05F513}
AppName={#MyAppName}
AppVersion={#GetEnv("GOPDF_VERSION")}
AppPublisher=GoPDF
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
LicenseFile=..\..\LICENSE
OutputDir=..\..\dist\installer
OutputBaseFilename=gopdf-{#GetEnv("GOPDF_VERSION")}-windows-amd64-installer
Compression=lzma
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\gopdf.ico
ChangesAssociations=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked
Name: "associatepdf"; Description: "Associate PDF files with GoPDF"; GroupDescription: "File associations:"; Flags: checkedonce
Name: "associatedocuments"; Description: "Offer GoPDF for XPS, e-book and comic book files"; GroupDescription: "File associations:"; Flags: checkedonce
Name: "associateimages"; Description: "Offer GoPDF for images"; GroupDescription: "File associations:"; Flags: unchecked

[Files]
Source: "..\..\dist\gopdf-package\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Registry]
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocExt}\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatepdf
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}"; ValueType: string; ValueName: ""; ValueData: "{#MyAppAssocName}"; Flags: uninsdeletekey; Tasks: associatepdf
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}"; ValueType: string; ValueName: "AppUserModelID"; ValueData: "{#MyAppUserModelID}"; Tasks: associatepdf
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\{#MyAppExeName},0"; Tasks: associatepdf
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: associatepdf
Root: HKA; Subkey: "Software\Classes\{#MyAppDocumentKey}"; ValueType: string; ValueName: ""; ValueData: "{#MyAppName} Document"; Flags: uninsdeletekey; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\{#MyAppDocumentKey}"; ValueType: string; ValueName: "AppUserModelID"; ValueData: "{#MyAppUserModelID}"; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\{#MyAppDocumentKey}\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\{#MyAppExeName},0"; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\{#MyAppDocumentKey}\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.xps\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.oxps\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.epub\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.mobi\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.prc\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.fb2\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.cbz\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\.cbt\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppDocumentKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associatedocuments
Root: HKA; Subkey: "Software\Classes\{#MyAppImageKey}"; ValueType: string; ValueName: ""; ValueData: "{#MyAppName} Image"; Flags: uninsdeletekey; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\{#MyAppImageKey}"; ValueType: string; ValueName: "AppUserModelID"; ValueData: "{#MyAppUserModelID}"; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\{#MyAppImageKey}\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\{#MyAppExeName},0"; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\{#MyAppImageKey}\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.png\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.jpg\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.jpeg\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.tif\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.tiff\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.gif\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.bmp\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.jp2\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.psd\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages
Root: HKA; Subkey: "Software\Classes\.svg\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppImageKey}"; ValueData: ""; Flags: uninsdeletevalue; Tasks: associateimages

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; AppUserModelID: "{#MyAppUserModelID}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; AppUserModelID: "{#MyAppUserModelID}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Parameters: "-v"; Flags: runhidden
