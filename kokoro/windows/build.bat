goto :start
Copyright (C) 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Windows Build Script.

:start
dir "%ProgramFiles%"
dir "%ProgramFiles(x86)%"
EXIT /B

set BUILD_ROOT=%cd%
set SRC=%cd%\github\agi

REM Use a fixed JDK.
set JAVA_HOME=c:\Program Files\Java\jdk1.8.0_144

REM Install the Android SDK components and NDK.
set ANDROID_HOME=%LOCALAPPDATA%\Android\Sdk

REM Install a license file for the Android SDK to avoid license query.
REM This file might need to be updated in the future.
copy /Y "%SRC%\kokoro\windows\android-sdk-license" "%ANDROID_HOME%\licenses\"

REM Install Android SDK platform, build tools and NDK
setlocal
call %ANDROID_HOME%\tools\bin\sdkmanager.bat platforms;android-26 build-tools;29.0.2
endlocal
echo on

wget -q https://dl.google.com/android/repository/android-ndk-r21d-windows-x86_64.zip
unzip -q android-ndk-r21d-windows-x86_64.zip
set ANDROID_NDK_HOME=%CD%\android-ndk-r21d

REM Install WiX Toolset.
wget -q https://github.com/wixtoolset/wix3/releases/download/wix311rtm/wix311-binaries.zip
unzip -q -d wix wix311-binaries.zip
set WIX=%cd%\wix

wget -q https://github.com/msys2/msys2-installer/releases/download/2020-11-09/msys2-base-x86_64-20201109.sfx.exe
.\msys2-base-x86_64-20201109.sfx.exe -y -o%BUILD_ROOT%\

wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-gcc-10.2.0-5-any.pkg.tar.zst
wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-gcc-libs-10.2.0-5-any.pkg.tar.zst
%BUILD_ROOT%\msys64\usr\bin\bash --login -c "pacman -Q"
%BUILD_ROOT%\msys64\usr\bin\bash --login -c "pacman -S --noconfirm git patch"
%BUILD_ROOT%\msys64\usr\bin\bash --login -c "pacman -U --noconfirm /t/src/mingw-w64-x86_64-gcc-10.2.0-5-any.pkg.tar.zst /t/src/mingw-w64-x86_64-gcc-libs-10.2.0-5-any.pkg.tar.zst"
set PATH=%BUILD_ROOT%\msys64\mingw64\bin;%BUILD_ROOT%\msys64\usr\bin;%PATH%
set BAZEL_SH=%BUILD_ROOT%\msys64\usr\bin\bash

REM Manually install only the required MSYS packages. Do NOT do a
REM system update (pacman -Syu) because it is a moving target.
REM First update pacman to support packages compressed with zstd.
REM c:\tools\msys64\usr\bin\bash --login -c "pacman -Sydd --noconfirm pacman"
REM c:\tools\msys64\usr\bin\bash --login -c "pacman -R --noconfirm catgets libcatgets"
REM Use an old version of patch known to work with the msys runtime
REM version that comes on Kokoro. Temporarily getting it from an
REM alternate mirror, since they msys host no longer carries this
REM version.
REM wget -q http://ftp.oregonstate.edu/pub/xbmc/build-deps/win32/msys2/repos/msys2/x86_64/patch-2.7.5-1-x86_64.pkg.tar.xz
REM c:\tools\msys64\usr\bin\bash --login -c "pacman -v -U --noconfirm  /t/src/patch-2.7.5-1-x86_64.pkg.tar.xz"
REM wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-binutils-2.35.1-3-any.pkg.tar.zst
REM c:\tools\msys64\usr\bin\bash --login -c "pacman -U --noconfirm /t/src/mingw-w64-x86_64-binutils-2.35.1-3-any.pkg.tar.zst"
REM wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-gcc-10.2.0-5-any.pkg.tar.zst
REM wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-gcc-libs-10.2.0-5-any.pkg.tar.zst
REM c:\tools\msys64\usr\bin\bash --login -c "pacman -U --noconfirm /t/src/mingw-w64-x86_64-gcc-10.2.0-5-any.pkg.tar.zst /t/src/mingw-w64-x86_64-gcc-libs-10.2.0-5-any.pkg.tar.zst"
REM wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-crt-git-9.0.0.6029.ecb4ff54-1-any.pkg.tar.zst
REM c:\tools\msys64\usr\bin\bash --login -c "pacman -U --noconfirm /t/src/mingw-w64-x86_64-crt-git-9.0.0.6029.ecb4ff54-1-any.pkg.tar.zst"
REM set PATH=c:\tools\msys64\mingw64\bin;c:\tools\msys64\usr\bin;%PATH%
REM set BAZEL_SH=c:\tools\msys64\usr\bin\bash

REM Get the JDK from our mirror.
set JDK_BUILD=zulu8.46.0.19-ca
set JDK_VERSION=8.0.252
set JDK_NAME=%JDK_BUILD%-jdk%JDK_VERSION%-win_x64
set JRE_NAME=%JDK_BUILD%-jre%JDK_VERSION%-win_x64
wget -q https://storage.googleapis.com/jdk-mirror/%JDK_BUILD%/%JDK_NAME%.zip
echo "993ef31276d18446ef8b0c249b40aa2dfcea221a5725d9466cbea1ba22686f6b  %JDK_NAME%.zip" | sha256sum --check
unzip -q %JDK_NAME%.zip
set JAVA_HOME=%CD%\%JDK_NAME%

wget -q https://storage.googleapis.com/jdk-mirror/%JDK_BUILD%/%JRE_NAME%.zip
echo "cf5cc2b5bf1206ace9b035dee129a144eda3059f43f204a4ba5e6911d95f0d0c  %JRE_NAME%.zip" | sha256sum --check
unzip -q %JRE_NAME%.zip
set JRE_HOME=%CD%\%JRE_NAME%

REM Install Bazel.
set BAZEL_VERSION=2.0.0
wget -q https://github.com/bazelbuild/bazel/releases/download/%BAZEL_VERSION%/bazel-%BAZEL_VERSION%-windows-x86_64.zip
unzip -q bazel-%BAZEL_VERSION%-windows-x86_64.zip
set PATH=C:\python27;%PATH%

cd %SRC%

REM Invoke the build.
echo %DATE% %TIME%
if "%KOKORO_GITHUB_COMMIT%." == "." (
  set BUILD_SHA=%DEV_PREFIX%%KOKORO_GITHUB_PULL_REQUEST_COMMIT%
) else (
  set BUILD_SHA=%DEV_PREFIX%%KOKORO_GITHUB_COMMIT%
)

%BUILD_ROOT%\bazel build -c opt --config symbols ^
    --define AGI_BUILD_NUMBER="%KOKORO_BUILD_NUMBER%" ^
    --define AGI_BUILD_SHA="%BUILD_SHA%" ^
    //gapis/api/vulkan:go_default_library
if %ERRORLEVEL% GEQ 1 exit /b %ERRORLEVEL%

REM Build everything else.
%BUILD_ROOT%\bazel build -c opt --config symbols ^
    --define AGI_BUILD_NUMBER="%KOKORO_BUILD_NUMBER%" ^
    --define AGI_BUILD_SHA="%BUILD_SHA%" ^
    //:pkg //:symbols //cmd/smoketests //cmd/vulkan_sample:vulkan_sample //tools/logo:agi_ico
if %ERRORLEVEL% GEQ 1 exit /b %ERRORLEVEL%
echo %DATE% %TIME%

REM Smoketests
%SRC%\bazel-bin\cmd\smoketests\windows_amd64_stripped\smoketests -gapit bazel-bin\pkg\gapit -traces test\traces
echo %DATE% %TIME%

REM Build the release packages.
mkdir %BUILD_ROOT%\out
call %SRC%\kokoro\windows\package.bat %BUILD_ROOT%\out %SRC%
