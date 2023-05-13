## benchmark

EU0D T7 @ 25mhz on bench, 14 symbols

CANUSB 96 - 102 fps
SM2 PRO 109 - 119 fps
OBDLinkSX 10 - 15 fps
CombiAdapter 100 - 106 fps
Mongoose Pro GM II 101 - 106 fps
STN2120 97 - 103 fps


## build
$env:PKG_CONFIG_PATH="C:\vcpkg\packages\libusb_x86-windows\lib\pkgconfig"; $env:CGO_CFLAGS="-IC:\vcpkg\packages\libusb_x86-windows\include\libusb-1.0"; $env:GOARCH=386; $env:CGO_ENABLED=1; go run -tags combi .

## run
$env:PKG_CONFIG_PATH="C:\vcpkg\packages\libusb_x86-windows\lib\pkgconfig"; $env:CGO_CFLAGS="-IC:\vcpkg\packages\libusb_x86-windows\include\libusb-1.0"; $env:GOARCH=386; $env:CGO_ENABLED=1; fyne package -tags combi --release
