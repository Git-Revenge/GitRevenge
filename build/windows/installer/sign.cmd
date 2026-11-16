@echo off
rem Signs the file in %1 with the Certum code signing certificate whose SHA-1 thumbprint is in
rem GITREVENGE_SIGN_SHA1. project.nsi runs it on the app, the uninstaller and the installer.
rem Release builds set the variable once SimplySign Desktop is signed in (see release.yml).
rem Without it, as in local and pull request builds, it does nothing.
if "%GITREVENGE_SIGN_SHA1%"=="" exit /b 0

rem The timestamp server sometimes fails a request, so try a few times.
for /l %%i in (1,1,3) do (
  signtool sign /sha1 %GITREVENGE_SIGN_SHA1% /fd SHA256 /tr http://time.certum.pl /td SHA256 /d "Git Revenge" /du https://gitrevenge.com %1 && exit /b 0
  echo Signing %1 failed, attempt %%i of 3
  timeout /t 10 /nobreak >nul
)
exit /b 1
