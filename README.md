# tgautodown
一款用于下载TG上视频、音乐、图片、文档、磁力链的TG机器人；只需将所需的资源转发给机器人，机器人会自动完成下载，并且会在完成后回复消息通知你。

# TG截图
- 下载视频
![视频下载截图](https://github.com/nasbump/tgautodown/blob/master/screenshots/download-video.png)

- 下载音乐
![音乐下载截图](https://github.com/nasbump/tgautodown/blob/master/screenshots/download-audio.png)

- 下载图片
![图片下载截图](https://github.com/nasbump/tgautodown/blob/master/screenshots/download-photos.png)

- 下载磁力链
![磁力链下载截图](https://github.com/nasbump/tgautodown/blob/master/screenshots/download-magnet.png)

- 下载文档
![文档下载截图](https://github.com/nasbump/tgautodown/blob/master/screenshots/download-docs.png)

- 笔记摘抄
![笔记摘抄截图](https://github.com/nasbump/tgautodown/blob/master/screenshots/download-note.png)

# 编译安装
项目纯go实现，直接拉代码编译：
```
git clone https://github.com/nasbump/tgautodown.git
cd tgautodown
go build
```

# 启动
```
$ ./build/tgautodown -h
Usage of ./build/tgautodown:
  -dir string
        save dir (default "./")
  -gopeed string
        gopeed path
  -token string
        tg-bot-token
```
下载依赖 gopeed[https://github.com/GopeedLab/gopeed] 下载器
gopeed编译方法：
```
go install github.com/GopeedLab/gopeed/cmd/gopeed@latest
```
### 启动示例：
```
./tgautodown -token 123456789:AAABBBCCCDDDEEFF -gopeed /usr/bin/gopeed -dir /data/data/files
```
### 下载保存路径：
视频、音乐、文档、图片、磁力链、笔记分别保存在`video`,`audio`,`doc`,`photos`,`bt`,`note`目录下:
```
$ tree /data/data/files/
/data/data/files/
├── audio
│   ├── 231
│   └── 234
│       └── file_1.mp3
├── bt
│   └── 251
│       └── Flow.2024.1080p.WEBRip.10Bit.DDP5.1.x265-Asiimov
│           ├── Flow.2024.1080p.WEBRip.10Bit.DDP5.1.x265-Asiimov.mkv
│           └── HDRush.cc.txt
├── doc
│   ├── 243
│   │   └── file_7.pdf
│   └── 248
│       └── file_8.JPG
├── note
│   └── 246.md
├── photos
│   └── 240
│       ├── file_3.jpg
│       ├── file_4.jpg
│       ├── file_5.jpg
│       └── file_6.jpg
└── video
    └── 237
        └── file_2.mp4
```

# Docker安装
- 将编译好的 `tgautodown`,`gopeed`，放到bin目录下
- 执行 `docker build -t tgautodown:latest .` 构建docker镜像
- 运行 `docker run -d --net host -v <下载路径>:/download -e TGBOTOKEN=<自己的机器人token> tgautodown:latest`

# 感谢
- [gopeed](https://github.com/GopeedLab/gopeed)
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)