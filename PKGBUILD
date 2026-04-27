# Maintainer: nyankopc <your@email.com>

pkgname=dispeys
pkgver=0.1.0
pkgrel=1
pkgdesc='Linux tray application that controls Ulanzi D200 stream deck devices with custom button configurations based on the active window'
arch=('x86_64')
url='https://github.com/krusherpt/dispeys'
license=('MIT')
depends=('xdotool' 'xprop' 'wmctrl' 'gtk3')
makedepends=('go' 'gcc')
optdepends=('nvidia-utils: GPU usage monitoring via nvidia-smi')
source=("$_pkgname-$pkgver.tar.gz::https://github.com/krusherpt/dispeys/archive/refs/tags/v$pkgver.tar.gz")
sha256sums=('SKIP')

prepare() {
  cd "$pkgname-$pkgver"
}

build() {
  cd "$pkgname-$pkgver"
  CGO_ENABLED=1 go build -o dispeysController -a -gcflags="all=-l -B" -ldflags="-s -w" cmd/controller/main.go
}

package() {
  # Install binary to /usr/bin
  install -Dm755 -t "$pkgdir/usr/bin" "$srcdir/$pkgname-$pkgver/dispeysController"

  # Install systemd user service
  install -Dm644 -t "$pkgdir/usr/lib/systemd/user" "$srcdir/$pkgname-$pkgver/dispeys.service"
}
