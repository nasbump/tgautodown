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
  -cfg     ## 配置文件，默认为: /app/data/config.json
  -proxy   ## socks5代理地址: 127.0.0.1:1080
  -f2a     ## TG账号开启了两步认证的话，这里需要输入密码
  -retrycnt 10  ## 失败时最大重试次数
  -names   ## 频道名，支持公开频道和私有频道
           ## 可以传多个频道，以,号隔; 如 -names abc,+def 这表示接收公共频道abc和私有频道+def中的消息
           ## 建议使用自建的私有频道，减少噪音
```
- 配置文件示例
```
{
  "cfgdir":"/app/data", ## 配置文件保存目录
  "saveDir":"/app/download", ## 下载文件保存目录
  "gopeed":"/app/bin/gopeed", ## BT下载命令路径
  "httpaddr":":2020", ## web服务端口
}
```


### 启动示例：
```
./build/tgautodown \
  -proxy 192.168.1.7:7891 \
  -cfg /app/data/config.json \
  -names +AjbQIYhiKlhhNzMx  
```

### 首次启动需要登陆TG
- 浏览器打开： http://<IP>:2020
- 参考下图流程完成登陆，以后就可以不用再登陆了
![web登陆](https://github.com/nasbump/tgautodown/blob/main/screenshots/web_login.jpg)

### 下载保存路径：
视频、音乐、文档、图片、磁力链、笔记分别保存在`videos`,`music`,`documents`,`photos`,`bt`,`note`目录下。

- 其他
1. appid和apphash获取：https://core.telegram.org/api/obtaining_api_id

# docker启动
- docker-compose:
```
services:
  tgautodown:
    image: nasbump/tgautodown:latest
    container_name: tgautodown
    restart: unless-stopped
    environment:
      - TG_CHANNEL=+AjbQIYhiKlhhNzMx  # 频道名，私有频道的话一定要带上+号
      - TG_PROXY=socks5://192.168.31.2:7891 # 代理地址，目前只支持socks5代理
      - TG_F2A=f2apassword  # TG账号开启了两步认证的话，这里需要输入密码
      - TG_RETRYCNT=10    # 失败时最大重试次数
    ports:
      - 2020:2020
    volumes:
      - /mnt/sda1/download:/app/download
      - /mnt/sda1/data:/app/data
```


# 感谢
- [基于MTProto协议的TG库](github.com/gotd/td/tg)
- [下载：gopeed](https://github.com/GopeedLab/gopeed)
