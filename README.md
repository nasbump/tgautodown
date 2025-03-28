# tgautodown
一款用于下载TG上视频、音乐、图片、文档、磁力链的TG机器人；只需将所需的资源转发给机器人，机器人会自动完成下载，并且会在完成后回复消息通知你。

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
Usage of ./tgautodown:
  -botSrv string
        bot-api-server: http://localhost:8081
  -debug
        enable bot-api debug
  -dir string
        save dir (default "./")
  -gopeed string
        gopeed path
  -token string
        tg-bot-token
```

- 参数说明:

| 参数名 | 是否必须 | 说明 |
|--------|---------|---------|
| token | 是 | 机器人token，获取方法见下 |
| dir | 是 | 下载保存路径 |
| botSrv | 否 | 是否使用自建bot服务，注意：使用官方bot服务时无法下载超过50M的文件 |
| gopeed | 否 | 下载器，使用自建bot服务时可以不传该参数，使用官方bot服务时建议传，否则无法下载，只能记笔记摘要 |
| debug | 否| 用于打印调试日志 |

- token获取方法：
1. 在TG中搜索“BotFather”，然后点击“开始”与其进行对话。
2. 在与BotFather的对话中，输入“/newbot”并按照提示操作。我们需要为机器人取一个独特的名称和用户名（用户名必须以bot结尾）。
3. 不出意外的话，此时就会得到一个机器人Token。(注意：包含数字冒号字母一整串)

- 下载器的编译：[gopeed](https://github.com/GopeedLab/gopeed) 下载器
```
go install github.com/GopeedLab/gopeed/cmd/gopeed@latest
```

### 启动示例：
- 使用官方botSrv：
```
./tgautodown -token 123456789:AAABBBCCCDDDEEFF -gopeed /usr/bin/gopeed -dir /data/data/files
```
- 使用自建botSrv：
```
./tgautodown -token 123456789:AAABBBCCCDDDEEFF -gopeed /usr/bin/gopeed -dir /data/data/files -botSrv http://localhost:8081
```

### 下载保存路径：
视频、音乐、文档、图片、磁力链、笔记分别保存在`videos`,`music`,`documents`,`photos`,`bt`,`note`目录下。


# Docker安装
我已将编译好的 `tgautodown`,`gopeed`,以及`botSrv`都打成docker镜像并上传到了 [dockerhub](https://hub.docker.com/r/nasbump/tgautodown)
- docker run：
```
docker run -d --net host \
        --name tgautodown \
        -v <path-to-download-dir>:/download \
        -e TGBOT_TOKEN=<TG-Bot-Token> \
        -e TGBOT_API_ID=<TG-Bot-API-ID> \
        -e TGBOT_API_HASH=<TG-Bot-API-HASH> \
        nasbump/tgautodown:latest


```

- docker compose:
```
services: 
  tgautodown: 
    image: nasbump/tgautodown:latest  
    container_name: tgautodown 
    network_mode: host 
    environment: 
      - TGBOT_TOKEN=<TG-Bot-Token>
      - TGBOT_API_ID=<TG-Bot-API-ID>
      - TGBOT_API_HASH=<TG-Bot-API-HASH>
    volumes: 
      - <path-to-download-dir>:/download 
    restart: unless-stopped
```

- 参数说明
1. path-to-download-dir: 替换为自己的下载目录
2. TG-Bot-Token： 替换为自己TG机器人的token
3. TG-Bot-API-ID，TG-Bot-API-HASH：
      - 如果不指定，则表示使用官方bot服务
      - 如果指定了api-id, api-hash，则使用自建bot服务
      - 获取方式：https://core.telegram.org/api/obtaining_api_id⁠
4. 如果要配置代理，则增加: HTTP_PROXY=http://XXX 及 HTTPS_PROXY=http://XXX 两个环境变量

# 感谢
- [gopeed](https://github.com/GopeedLab/gopeed)
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)