#!/bin/sh
# Rebuilds onon's icon files in assets/ from the SVG sources here. Needs
# macOS with Xcode 26 or later (swiftc, iconutil and actool) and ImageMagick for
# the .ico and .bmp. AppKit draws the SVGs: ImageMagick's own SVG
# renderer gets the arcs wrong.
#
#   onon.svg (in assets/)       Linux scalable icon and 256 px PNG, SDL window icon
#   onon-macos*.svg             macOS 12-15 .icns, art on Apple's 824/1024 grid
#   onon-windows*.svg           Windows .ico: no plate, mid greens for any taskbar
#   ../macos/onon.icon          macOS 26 and later, compiled here into ../macos/Assets.car,
#                               since release runners may lack Xcode 26's actool
set -eu
cd "$(dirname "$0")"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
swiftc -O rasterize.swift -o "$work/rasterize"
r() { "$work/rasterize" "$1" "$2" "$3"; }

r ../onon.svg ../onon.png 256
r ../onon.svg "$work/bmp.png" 256
magick "$work/bmp.png" ../onon.bmp # SDL window icon, with alpha

for s in 16 24 32; do r onon-windows-small.svg "$work/win-$s.png" "$s"; done
for s in 48 64 256; do r onon-windows.svg "$work/win-$s.png" "$s"; done
magick "$work/win-16.png" "$work/win-24.png" "$work/win-32.png" "$work/win-48.png" "$work/win-64.png" "$work/win-256.png" ../onon.ico

set=$work/onon.iconset
mkdir "$set"
r onon-macos-small.svg "$set/icon_16x16.png" 16
r onon-macos-small.svg "$set/icon_16x16@2x.png" 32
r onon-macos-small.svg "$set/icon_32x32.png" 32
r onon-macos.svg "$set/icon_32x32@2x.png" 64
for s in 128 256 512; do
	r onon-macos.svg "$set/icon_${s}x${s}.png" "$s"
	r onon-macos.svg "$set/icon_${s}x${s}@2x.png" $((s * 2))
done
iconutil -c icns "$set" -o ../macos/onon.icns
r onon-macos.svg ../macos/onon-1024.png 1024
mkdir "$work/car"
xcrun actool ../macos/onon.icon --compile "$work/car" --app-icon onon --platform macosx \
	--minimum-deployment-target 12.0 --output-partial-info-plist "$work/car.plist" >/dev/null
cp "$work/car/Assets.car" ../macos/Assets.car
