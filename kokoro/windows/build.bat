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
mkdir C:\src\github
cd C:\src
set BUILD_ROOT=%cd%
set SRC=%cd%\github\agi
xcopy C:\tmpfs\src\github %BUILD_ROOT%\github /s /e /y >null
rem rm -r -f C:\tmpfs\src\github\agi
ls C:\tmpfs\src\github
wmic computersystem get TotalPhysicalMemory
wmic OS get TotalVirtualMemorySize
wmic OS get FreePhysicalMemory
wmic OS get FreeVirtualMemory

psinfo -h -d

REM Install WiX (https://wixtoolset.org/, used in package.bat to create ".msi")
mkdir wix
cd wix
wget -q https://github.com/wixtoolset/wix3/releases/download/wix3112rtm/wix311-binaries.zip
REM Using 'set /p' prints without CRLF newline characters, which sha256sum can't handle.
REM If sha256sum fails, 'exit /b 1' will terminate batch with error code 1.
echo | set /p placeholder="2c1888d5d1dba377fc7fa14444cf556963747ff9a0a289a3599cf09da03b9e2e wix311-binaries.zip" | sha256sum --check || exit /b 1
unzip -q wix311-binaries.zip

REM Grant read and execute access for WIX
icacls *.* /grant ContainerAdministrator:rx

set WIX=%cd%
cd ..

REM Use a fixed JDK.
set JAVA_HOME=C:\Program Files\Java\jdk1.8.0_211

REM Install the Android SDK components and NDK.
set ANDROID_HOME=c:\Android\android-sdk

REM Install a license file for the Android SDK to avoid license query.
REM This file might need to be updated in the future.
copy /Y "%SRC%\kokoro\windows\android-sdk-license" "%ANDROID_HOME%\licenses\"

REM Install Android SDK platform, build tools and NDK
setlocal
call %ANDROID_HOME%\tools\bin\sdkmanager.bat platforms;android-26 build-tools;30.0.3
endlocal
echo on

wget -q https://dl.google.com/android/repository/android-ndk-r21d-windows-x86_64.zip
echo | set /p placeholder="18335e57f8acab5a4acf6a2204130e64f99153015d55eb2667f8c28d4724d927 android-ndk-r21d-windows-x86_64.zip" | sha256sum --check || exit /b 1
unzip -q android-ndk-r21d-windows-x86_64.zip
set ANDROID_NDK_HOME=%CD%\android-ndk-r21d

REM Download and install MSYS2, because the pre-installed version is too old.
REM Do NOT do a system update (pacman -Syu) because it is a moving target.
wget -q https://repo.msys2.org/distrib/x86_64/msys2-base-x86_64-20240727.sfx.exe
echo | set /p placeholder="587b86c7e2b7e77d133a4422d4c9b0ae68312442b905277ea3d4ccd0e75cbdea msys2-base-x86_64-20240727.sfx.exe" | sha256sum --check || exit /b 1
.\msys2-base-x86_64-20240727.sfx.exe -y -o%BUILD_ROOT%\

REM Start empty shell to initialize MSYS2.
%BUILD_ROOT%\msys64\usr\bin\bash --login -c " "

REM Uncomment the following line to list all packages and versions installed in MSYS2.
REM %BUILD_ROOT%\msys64\usr\bin\bash --login -c "pacman -Q"

REM Install packages required by the build process.
%BUILD_ROOT%\msys64\usr\bin\bash --login -c "pacman -S --noconfirm git patch zip unzip"

REM Download and install specific compiler version.
wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-gcc-14.2.0-1-any.pkg.tar.zst
echo | set /p placeholder="851c5ebcc4f787bce96c309b5c0c85c9f537053765520180f72f343b94d86b3b mingw-w64-x86_64-gcc-14.2.0-1-any.pkg.tar.zst" | sha256sum --check || exit /b 1
wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-gcc-libs-14.2.0-1-any.pkg.tar.zst
echo | set /p placeholder="3ac4352d1a4dec21508a71d744ff055d5643b2937c799b11114a33066abfb963 mingw-w64-x86_64-gcc-libs-14.2.0-1-any.pkg.tar.zst" | sha256sum --check || exit /b 1
wget -q http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-binutils-2.42-2-any.pkg.tar.zst
echo | set /p placeholder="b5a95772f674f54494cd8e4dd0f8702295ceb9099a30741cb0a86a670f32f499 mingw-w64-x86_64-binutils-2.42-2-any.pkg.tar.zst" | sha256sum --check || exit /b 1
%BUILD_ROOT%\msys64\usr\bin\bash --login -c "pacman -U --noconfirm mingw-w64-x86_64-gcc-14.2.0-1-any.pkg.tar.zst mingw-w64-x86_64-gcc-libs-14.2.0-1-any.pkg.tar.zst mingw-w64-x86_64-binutils-2.42-2-any.pkg.tar.zst"

REM Configure build process to use the now installed MSYS2.
set PATH=%BUILD_ROOT%\msys64\mingw64\bin;%BUILD_ROOT%\msys64\usr\bin;%PATH%
set BAZEL_SH=%BUILD_ROOT%\msys64\usr\bin\bash

REM Get the JDK from our mirror.
set JDK_BUILD=zulu11.39.15-ca
set JDK_VERSION=11.0.7
set JDK_NAME=%JDK_BUILD%-jdk%JDK_VERSION%-win_x64
set JRE_NAME=%JDK_BUILD%-jre%JDK_VERSION%-win_x64
wget -q https://storage.googleapis.com/jdk-mirror/%JDK_BUILD%/%JDK_NAME%.zip
echo | set /p placeholder="1d947cb3a846d87aca12d4ae94e1b51e1079fdcd9e4faea9684909b8072f8ed4 %JDK_NAME%.zip" | sha256sum --check || exit /b 1
unzip -q %JDK_NAME%.zip
set JAVA_HOME=%CD%\%JDK_NAME%

wget -q https://storage.googleapis.com/jdk-mirror/%JDK_BUILD%/%JRE_NAME%.zip
echo | set /p placeholder="339f622f688b129de16876ce8ee252eac0ab7663d81596016abf2efacab01d86 %JRE_NAME%.zip" | sha256sum --check || exit /b 1
unzip -q %JRE_NAME%.zip
set JRE_HOME=%CD%\%JRE_NAME%

REM Install Bazel.
set BAZEL_VERSION=6.5.0
wget -q https://github.com/bazelbuild/bazel/releases/download/%BAZEL_VERSION%/bazel-%BAZEL_VERSION%-windows-x86_64.zip
echo | set /p placeholder="258c5d6546f870cd354f094ced9c77f46849281e017ab9d1c31c39c8c7c685dc bazel-%BAZEL_VERSION%-windows-x86_64.zip" | sha256sum --check || exit /b 1
unzip -q bazel-%BAZEL_VERSION%-windows-x86_64.zip
set PATH=C:\python35;%PATH%

cd %SRC%

REM Invoke the build.
echo %DATE% %TIME%
if "%KOKORO_GITHUB_COMMIT%." == "." (
  set BUILD_SHA=%DEV_PREFIX%%KOKORO_GITHUB_PULL_REQUEST_COMMIT%
) else (
  set BUILD_SHA=%DEV_PREFIX%%KOKORO_GITHUB_COMMIT%
)

REM Make Bazel operate under BUILD_ROOT (T:\src), where files are less
REM likely to be locked by system checks.
set BAZEL_OUTPUT_USER_ROOT=%BUILD_ROOT%\build
mkdir %BAZEL_OUTPUT_USER_ROOT%

REM Build in several steps in order to avoid running out of memory.

set BUILD_TARGETS=//core/vulkan/vk_virtual_swapchain/apk:VkLayer_VirtualSwapchain
set BUILD_TARGETS=%BUILD_TARGETS%;@com_github_golang_protobuf//proto:go_default_library @com_github_pkg_errors//:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//gapis/replay/builder:go_default_library //gapis/replay/value:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//core/app/status:go_default_library //core/context/keys:go_default_library //core/data:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//core/data/binary:go_default_library //core/data/dictionary:go_default_library //core/data/endian:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//core/data/id:go_default_library //core/data/protoconv:go_default_library //core/event/task:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//core/image:go_default_library //core/image/astc:go_default_library //core/image/etc:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//core/log:go_default_library //core/math/interval:go_default_library //core/math/u64:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//core/os/device:go_default_library //core/stream:go_default_library //core/stream/fmts:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//core/vulkan/loader:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//gapis/api/vulkan:go_default_library
set BUILD_TARGETS=%BUILD_TARGETS%;//cmd/vulkan_sample:vulkan_sample //tools/logo:agi_ico
set BUILD_TARGETS=%BUILD_TARGETS%;//:pkg-lib
set BUILD_TARGETS=%BUILD_TARGETS%;//:pkg-strings
set BUILD_TARGETS=%BUILD_TARGETS%;//cmd/gapis 
set BUILD_TARGETS=%BUILD_TARGETS%;//cmd/device-info //cmd/gapir/cc:gapir //cmd/gapit
set BUILD_TARGETS=%BUILD_TARGETS%;//cmd/agi 
set BUILD_TARGETS=%BUILD_TARGETS%;//:pkg

REM Loop through the build targets
for %%T in (%BUILD_TARGETS%) do (
    wmic OS get FreePhysicalMemory
    wmic OS get FreeVirtualMemory

    %BUILD_ROOT%\bazel ^
        --output_user_root=%BAZEL_OUTPUT_USER_ROOT% ^
        build -c opt ^
        --define AGI_BUILD_NUMBER="%KOKORO_BUILD_NUMBER%" ^
        --define AGI_BUILD_SHA="%BUILD_SHA%" ^
        %%T

    if %ERRORLEVEL% GEQ 1 exit /b %ERRORLEVEL%
    echo %DATE% %TIME%

    tasklist /fi "memusage gt 150000"
)

REM Smoketests
%BUILD_ROOT%\bazel ^
    --output_user_root=%BAZEL_OUTPUT_USER_ROOT% ^
    run -c opt ^
    --define AGI_BUILD_NUMBER="%KOKORO_BUILD_NUMBER%" ^
    --define AGI_BUILD_SHA="%BUILD_SHA%" ^
    //cmd/smoketests -- --traces test/traces
if %ERRORLEVEL% GEQ 1 exit /b %ERRORLEVEL%
echo %DATE% %TIME%

REM Build the release packages.
mkdir %BUILD_ROOT%\out
call %SRC%\kokoro\windows\package.bat %BUILD_ROOT%\out %SRC%
