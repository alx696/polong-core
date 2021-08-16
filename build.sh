#!/bin/sh
set -e

NAME="polong-core"

go mod tidy
go mod download

mkdir -p build

#echo "Linux"
#go build -o "build/${NAME}-linux"
#
#echo "Windows"
#sudo apt-get install -y gcc-multilib gcc-mingw-w64
#GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -o "build/${NAME}-windows"

echo "Android"
echo "NDK路径: ${ANDROID_NDK_HOME}"
if [ -z "$(which gomobile)" ]; then
  echo "没有安装gomobile"
  go get golang.org/x/mobile/cmd/gomobile
  gomobile init
else
  echo "已经安装gomobile"
fi
gomobile bind -target=android/arm64,android/arm -o "build/${NAME}.aar" ./kc ./qc
rm "build/${NAME}-sources.jar"
