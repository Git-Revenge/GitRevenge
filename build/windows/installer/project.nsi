Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
# The uninstall key is fixed, so it doesn't depend on the publisher's name. Before 1.0 it was
# "Ris AdamsGit Revenge"; winget and RemoveInstallBefore1_0 below rely on these names.
!define UNINST_KEY_NAME "GitRevenge"
!define OLD_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ris AdamsGit Revenge"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

# Sign the app before it's packed, then the uninstaller and the installer once they're written.
# sign.cmd does nothing unless GITREVENGE_SIGN_SHA1 is set, which only release builds do.
!ifdef ARG_WAILS_AMD64_BINARY
  !system 'sign.cmd "${ARG_WAILS_AMD64_BINARY}"' = 0
!endif
!uninstfinalize 'sign.cmd "%1"' = 0
!finalize 'sign.cmd "%1"' = 0

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
!endif # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

# Whose PATH the launchers' folder goes on: everyone's for the usual install, which runs as
# an administrator, and the current user's for one built to run without.
!if "${REQUEST_EXECUTION_LEVEL}" == "user"
  !define PATH_SCOPE "user"
!else
  !define PATH_SCOPE "machine"
!endif

# Puts $INSTDIR\bin on the PATH (ACTION "add") or takes it off ("remove"), then tells running
# programs the environment changed, so the next terminal opened has it. The app makes the edit
# (internal/pathenv), because NSIS can't safely: it reads a registry string into a
# 1024-character buffer, so a longer PATH would come back empty and be written back that way.
!macro gitrevenge.path ACTION
    ClearErrors
    ExecWait '"$INSTDIR\${PRODUCT_EXECUTABLE}" --installer-path ${ACTION} ${PATH_SCOPE} "$INSTDIR\bin"' $0
    ${If} ${Errors}
        DetailPrint "Couldn't ${ACTION} the PATH entry: ${PRODUCT_EXECUTABLE} didn't start"
    ${ElseIf} $0 != 0
        DetailPrint "Couldn't ${ACTION} the PATH entry (exit code $0)"
    ${Else}
        SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000
    ${EndIf}
!macroend

# Versions before 1.0 installed into "Ris Adams\Git Revenge" under another uninstall key.
# Upgrading from one runs its own uninstaller quietly, from wherever it was installed, so
# Apps & features lists one Git Revenge. Settings live in %APPDATA% and aren't touched.
Function RemoveInstallBefore1_0
    SetRegView 64
    ReadRegStr $0 HKLM "${OLD_UNINST_KEY}" "UninstallString"
    StrCmp $0 "" done
    StrCpy $0 $0 -1 1 # drop the quotes around the path
    ${GetParent} $0 $1
    DetailPrint "Removing the earlier install in $1"
    IfFileExists $0 0 +2
        ExecWait '"$0" /S _?=$1'
    # With _?= the uninstaller runs in place and leaves itself and its folder behind.
    Delete $0
    RMDir $1
    ${GetParent} $1 $2
    RMDir $2 # the "Ris Adams" folder, only if nothing else is in it
    DeleteRegKey HKLM "${OLD_UNINST_KEY}"
  done:
FunctionEnd

Section
    !insertmacro wails.setShellContext

    Call RemoveInstallBefore1_0

    !insertmacro wails.webview2runtime

    # What a terminal runs as "gitrevenge": a .cmd for cmd and PowerShell, and a script for Git
    # Bash, which doesn't run a .cmd by its bare name. They have a folder of their own to go on
    # the PATH, so that uninstall.exe doesn't become a command as well.
    SetOutPath "$INSTDIR\bin"
    File "launcher\gitrevenge.cmd"
    File "launcher\gitrevenge"

    SetOutPath $INSTDIR

    ClearErrors
    !insertmacro wails.files
    # The error flag is set when the app couldn't be written and that was ignored, which leaves
    # the copy from before. The install stops there: this installer and its uninstaller run the
    # app to edit the PATH, and an older copy would take that as an ordinary start.
    ${If} ${Errors}
        Abort "Git Revenge couldn't be replaced. Close it and run this installer again."
    ${EndIf}
    !insertmacro gitrevenge.path "add"

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    # Before the app is deleted, since it's what takes the entry off.
    !insertmacro gitrevenge.path "remove"

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
