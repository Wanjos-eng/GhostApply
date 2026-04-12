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
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

RequestExecutionLevel admin
SetCompressor /SOLID lzma
XPStyle on
WindowIcon on

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
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_RIGHT
!define MUI_HEADERIMAGE_BITMAP "resources\header.bmp"
!define MUI_HEADER_TRANSPARENT_TEXT
!define MUI_WELCOMEFINISHPAGE_BITMAP "resources\welcome.bmp"
!define MUI_UNWELCOMEFINISHPAGE_BITMAP "resources\unwelcome.bmp"
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.
!define MUI_LICENSEPAGE_CHECKBOX
!define MUI_FINISHPAGE_LINK "Privacy Policy & Terms"
!define MUI_FINISHPAGE_LINK_LOCATION "https://github.com/Wanjos-eng/GhostApply"
!define MUI_WELCOMEPAGE_TITLE "Instalador GhostApply"
!define MUI_WELCOMEPAGE_TEXT "Bem-vindo ao instalador oficial do GhostApply.\r\n\r\nEsta instalacao prepara aplicativo, automacao e base local no padrao da plataforma para iniciar sem configuracao manual.\r\n\r\nClique em Avancar para continuar."
!define MUI_FINISHPAGE_TITLE "GhostApply instalado com sucesso"
!define MUI_FINISHPAGE_TEXT "A instalacao foi concluida com sucesso. O GhostApply esta pronto para uso."
!define MUI_COMPONENTSPAGE_TEXT_TOP "Selecione os itens opcionais da instalacao."
!define MUI_COMPONENTSPAGE_TEXT_COMPLIST "Componentes disponiveis"
!define MUI_COMPONENTSPAGE_SMALLDESC
!define MUI_DIRECTORYPAGE_TEXT_TOP "Escolha a pasta de instalacao do GhostApply."
!define MUI_INSTFILESPAGE_FINISHHEADER_TEXT "Configuracao concluida"
!define MUI_INSTFILESPAGE_FINISHHEADER_SUBTEXT "Todos os arquivos foram instalados corretamente."
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Abrir GhostApply agora"
!define MUI_FINISHPAGE_SHOWREADME "$INSTDIR\privacy.txt"
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Visualizar politica de privacidade"
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!define MUI_UNCONFIRMPAGE_TEXT_TOP "Esta acao removera o GhostApply e os arquivos instalados nesta pasta."
!define MUI_UNFINISHPAGE_TITLE "GhostApply removido"
!define MUI_UNFINISHPAGE_TEXT "A desinstalacao foi concluida com sucesso."

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
!insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_COMPONENTS # Optional components page.
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page
!insertmacro MUI_UNPAGE_FINISH

!insertmacro MUI_LANGUAGE "PortugueseBR" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES32\${INFO_PRODUCTNAME}" # Instala preferencialmente em Program Files (x86).
ShowInstDetails nevershow
ShowUnInstDetails nevershow
BrandingText "GhostApply | Instalacao oficial"

!define APP_DATA_DIR "$APPDATA\GhostApply"

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Section "Nucleo do aplicativo (obrigatorio)" SEC_CORE
    SectionIn RO
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files
        File "..\..\bin\filler.exe"
    File "resources\eula.txt"
    File "resources\privacy.txt"

        # Prepara dados mutaveis em AppData (padrao Windows para SQLite/WAL).
        CreateDirectory "${APP_DATA_DIR}"

        IfFileExists "${APP_DATA_DIR}\.env" env_exists env_missing
        env_missing:
            SetOutPath "${APP_DATA_DIR}"
            File /oname=.env "resources\app.env"
        env_exists:

        IfFileExists "${APP_DATA_DIR}\forja_ghost.sqlite" db_exists db_missing
        db_missing:
            SetOutPath "${APP_DATA_DIR}"
            File /oname=forja_ghost.sqlite "resources\forja_ghost.sqlite"
        db_exists:

        SetOutPath $INSTDIR

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section "Atalho na area de trabalho" SEC_DESKTOP
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_CORE} "Componentes obrigatorios do GhostApply: app, filler, banco local e configuracoes iniciais."
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_DESKTOP} "Cria um atalho do GhostApply na area de trabalho."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    Delete "$INSTDIR\eula.txt"
    Delete "$INSTDIR\privacy.txt"
    Delete "$INSTDIR\filler.exe"

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
