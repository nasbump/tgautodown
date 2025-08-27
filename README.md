# tgautodown
一款用于下载TG上视频、音乐、图片、文档、磁力链的TG机器人；只需将所需的资源转发给机器人，机器人会自动完成下载，并且会在完成后回复消息通知你。

### 最新版本全新升级
- 采用全新架构，完全重写
- 不再依赖机器人
- 不再依赖bot-api-server
- 支持获取视频、文档、音乐原文件名

# TG截图
- 下载视频
![视频下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-video.png)

- 下载音乐
![音乐下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-audio.png)

- 下载图片
![图片下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-photos.png)

- 下载磁力链
![磁力链下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-magnet.png)

- 下载文档
![文档下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-docs.png)

- 笔记摘抄
![笔记摘抄截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-note.png)

# 编译安装
项目纯go实现，直接拉代码编译：
```
git clone https://github.com/nasbump/tgautodown.git
cd tgautodown
go build
```

# 启动
- 启动参数：
```
$ ./tgautodown -h
usage: ./build/tgautodown options
  -phone   ## 登陆TG的手机号，用于接收验证码
  -appid   ## https://core.telegram.org/api/obtaining_api_id
  -apphash
  -proxy   ## socks5代理地址: 127.0.0.1:1080
  -names   ## 频道名，支持公开频道和私有频道; 建议使用自建的私有频道，减少噪音
  -session ./session.json  ## 会话保存文件，首次启动需要登陆，之后基于该文件不再需要登陆
  -gopeed   ## gopeed path
  -dir     ## 文件下载保存目录
  -logpath
  -logcnt 1
  -loglev 0  ## -1 trace, 0 debug, 1 info, 2 warn, 3 error, 5 fatal, 5 panic, 6 nolevel
  -logsize 52428800  ## MB
```


### 启动示例：
```
./build/tgautodown \
  -proxy 192.168.1.7:7891 \
  -phone +13057359985 \
  -appid 16103380 -apphash bdd26eb6a54fb59d75d9b2c47ee6eee7 \
  -session ./data/session.json \
  -names +AjbQIYhiKlhhNzMx  
```

### 下载保存路径：
视频、音乐、文档、图片、磁力链、笔记分别保存在`videos`,`music`,`documents`,`photos`,`bt`,`note`目录下。

- 其他
1. appid和apphash获取：https://core.telegram.org/api/obtaining_api_id

# 感谢
- [基于MTProto协议的TG库](github.com/gotd/td/tg)
- [下载：gopeed](https://github.com/GopeedLab/gopeed)
