# 破笼核心

破笼核心，提供主要功能，提供功能接口。Android中直接调用原生接口，Web和桌面端中通过http(websocket)接口调用。

## 开发

[基于go-libp2p](https://github.com/libp2p/go-libp2p)，使用Golang语言开发，开发工具推荐使用[Visual Studio Code](https://code.visualstudio.com/)。

### Android

[破笼 Android](https://github.com/alx696/polong-android)，使用[mobile](https://pkg.go.dev/golang.org/x/mobile)生成AAR，提供破笼Android端。

项目文件夹中执行下面命令生成AAR:
```
$ gomobile bind -target=android/arm64,android/arm -v -o /path/to/android/project/gomobile/gomobile.aar ./kc ./qc
```

### Web

[破笼 Web](https://github.com/alx696/polong-web)，提供浏览器和桌面端的界面。

### 桌面端

[破笼 桌面端](https://github.com/alx696/polong-desktop)，使用[electron](https://www.electronjs.org/)和[electron-builder](https://www.electron.build/)打包集成后端和界面，提供破笼Linux(DEB,RPM)，Windows客户端。