#!/bin/bash

APP_NAME="pheri"
VERSION="1.0.0"
MAINTAINER="Vicky Chhetri <admin@vickychhetri.com>"

mkdir -p build

# 1. Build for Linux
echo "🔨 Building Linux binaries..."
GOOS=linux GOARCH=amd64 go build -o build/${APP_NAME}-linux-amd64 main.go
GOOS=linux GOARCH=arm64 go build -o build/${APP_NAME}-linux-arm64 main.go

# 2. Build for Windows
echo "🔨 Building Windows binaries..."
GOOS=windows GOARCH=amd64 go build -o build/${APP_NAME}-windows-amd64.exe main.go
GOOS=windows GOARCH=arm64 go build -o build/${APP_NAME}-windows-arm64.exe main.go

# 3. Package tar.gz (Linux)
echo "📦 Packaging Linux tar.gz..."
tar -czvf build/${APP_NAME}-linux-amd64.tar.gz -C build ${APP_NAME}-linux-amd64
tar -czvf build/${APP_NAME}-linux-arm64.tar.gz -C build ${APP_NAME}-linux-arm64

# 4. Package zip (Windows)
echo "📦 Packaging Windows zip..."
zip -j build/${APP_NAME}-windows-amd64.zip build/${APP_NAME}-windows-amd64.exe
zip -j build/${APP_NAME}-windows-arm64.zip build/${APP_NAME}-windows-arm64.exe

# 5. Build .deb for amd64
echo "📦 Building .deb package..."
DEB_DIR=build/${APP_NAME}_${VERSION}_amd64
mkdir -p ${DEB_DIR}/DEBIAN
mkdir -p ${DEB_DIR}/usr/local/bin

cp build/${APP_NAME}-linux-amd64 ${DEB_DIR}/usr/local/bin/${APP_NAME}
chmod 755 ${DEB_DIR}/usr/local/bin/${APP_NAME}

cat <<EOF > ${DEB_DIR}/DEBIAN/control
Package: ${APP_NAME}
Version: ${VERSION}
Section: base
Priority: optional
Architecture: amd64
Maintainer: ${MAINTAINER}
Description: Pheri is a terminal-based user interface (TUI) for MySQL. It allows you to connect to your MySQL databases and interact with them directly from your terminal.
EOF

dpkg-deb --build ${DEB_DIR}
mv ${DEB_DIR}.deb build/${APP_NAME}_${VERSION}_amd64.deb

echo "✅ Build complete! Files:"
ls -lh build/