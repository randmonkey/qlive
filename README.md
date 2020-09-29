# qlive
七牛互动直播demo 后端

### 功能
- 使用手机短信验证码登录互动直播服务
- 开启直播间进行直播
- 进入直播间观看直播，并进行实时聊天、发送弹幕、赠送礼物等互动
- 主播与其他直播间的主播进行连麦PK互动，观众可观看两主播的PK

### 项目构建
本项目使用go mod方式编译。
```
export GO111MODULE=on
git@github.com:qrtc/qlive.git
go mod tidy
go build .
```

### 运行

完成构建，并编辑好配置文件（如`qlive.conf`，配置文件详细说明见下文），然后运行：
```
./qlive -f qlive.conf
```
即可运行本互动直播后端服务。

### 开发

#### 添加license与版权声明注释
vscode：可使用licenser插件添加注释。安装licenser插件后，编辑`.vscode/settings.json`文件，添加如下两行：
```
{
    ...
    "licenser.projectName": "qlive",
    "licenser.author": "Qiniu Cloud (qiniu.com)",
}
```
新建的文件将自动添加注释，原有文件可使用 `licenser: Insert license header` 命令添加注释（Windows/Linux系统快捷键`Ctrl+Shift+P` ，Mac OS 快捷键 `⌘⇧P`(`Command+Shift+P`) 

### 依赖
本互动直播后端需要依赖mongo数据库储存服务状态。
运行本互动直播后端需要使用以下服务：
- 七牛云短信服务
- 七牛云实时音视频（RTC）服务
- 融云即时通信（IM）服务

### API说明
本项目的API文档通过swagger生成，可使用如下命令使用docker在本地生成API文档站：
```
 # QIVE_ROOT为本项目所在目录的绝对路径
 docker run -p 80:8080 -e SWAGGER_JSON=/docs/swagger.json -v ${QLIVE_ROOT}/docs:/docs swaggerapi/swagger-ui
```
然后通过127.0.0.1访问文档站。

或者启动服务之后，通过`<listen_addr>/swagger/index.html`访问API文档 (`<listen_addr>`为HTTP服务实际监听的地址)。

### 配置文件说明

互动直播后端的配置文件样例见 `qlive.conf`，相关配置项的定义可参见`config/config.go`。配置文件分为以下部分：

HTTP服务配置

本服务通过HTTP API进行用户登录、创建直播间、进入直播间等操作。
```
    "debug_level":0,
    "listen_addr": ":8080",
```
`debug_level` 表示日志输出的等级，设为1表示仅输出INFO级别及以上的日志，不输出DEBUG日志；设为0表示输出DEBUG日志。

`listen_addr` 表示HTTP监听的地址。

Websocket信令服务配置

本服务通过websocket长连接实现保活探测，PK请求与结果的接收和推送等功能。
配置文件中的`websocket_conf`部分为websocket服务的配置。
```
    "websocket_conf":{
        "listen_addr":":8082",
        "serve_uri": "/qlive",
        "ws_over_tls": false,
#       "external_ws_url":"ws://example.com/qlive" # uncomment this line 
        "authorize_timeout_ms": 5000,
        "ping_ticker_s": 5,
        "pong_timeout_s": 20
    }
``` 
`listen_addr` 表示websocket服务监听的地址。

`serve_uri` 表示websocket服务监听的URL路径。
 
`ws_over_tls` 表示是否对外提供wss协议的websocket服务。

`external_ws_url` 表示对外提供的websocket服务地址，客户端需要使用这个地址与服务端建立websocket连接。

`authorize_timeout_ms` 表示客户端建立websocket连接后进行认证的超时时间，单位为毫秒。超过这个时间未完成认证，websocket连接将自动断开。

`ping_ticker_s` 表示服务端发送ping/pong保活探测消息的时间间隔，单位为秒。每经过一个该间隔的时间，服务端将发送一个ping消息，客户端回应一个pong消息表示自己还在线。

`pong_timeout_s` 表示服务端的连接超时时间，单位为秒。若服务端在该时间内未接收到客户端的任何消息，将断开与客户端的websocket连接。

Mongo数据库配置

本服务使用mongo储存用户信息、用户登录状态、房间状态等服务状态信息。
配置文件中的`mongo`部分表示使用mongo数据库的配置。
```
    "mongo":{
        "uri":"mongodb://127.0.0.1:27017",
        "database": "qrtc_qlive_test"
    }
```

`uri` 表示mongo数据库的URI。`database`表示使用的mongo数据库名称。

短信服务配置

本服务使用手机短信发送验证码，用于用户登录使用。`sms`部分为短信服务配置。
```
    "sms":{
        "provider":"qiniu",
        "qiniu_sms":{
            "key_pair":{
                "access_key": "ak", # replace with real ak/sk
                "secert_key": "sk"
            },
            "signature_id":"sign_id", # replace with real singature ID and template ID
            "template_id":"temp_id"
        }
    }
```
`provider` 为短信服务提供商，目前支持`qiniu`表示使用七牛云短信提供短信服务。也可在进行本地开发调试时修改成`test`，实际不发送短信。

`qiniu_sms` 部分为七牛云短信的配置。其中，`key_pair`部分为您使用的七牛云AK/SK密钥对。`signature_id` 为七牛云短信中验证码短信的签名ID，`template_id` 为七牛云短信中验证码短信的模板ID。签名ID与模板ID可以在七牛云短信的控制台中查看。

RTC与直播服务配置

本服务使用七牛RTC与直播服务为提供推流、PK连麦、合流转推至RTMP流等服务。`rtc`部分为RTC与直播服务配置。
```
    "rtc":{
        "key_pair":{
            "access_key": "ak", # replace with real ak/sk
            "secert_key": "sk"
        },
        "app_id":"app", # replace with real RTC app ID
        "publish_host": "live.qiniu.com", # replace with real publish host & hubs
        "publish_hub": "testhub"
    }
```

`key_pair` 同上，为您使用七牛RTC与直播服务使用的七牛云AK/SK密钥对。`app_id` 为您使用的RTC应用ID。`publish_host`与`publish_hub`为您使用的直播推流域名和hub。

IM服务配置

本服务使用即时通信（IM）服务提供聊天、发送弹幕、赠送礼物等互动功能。`im`部分表示IM服务的配置。
```
    "im":{
        "provider":"rongcloud",
        "rongcloud":{
            "app_key":"ak", #replace with real rongcloud appkey/appsecret
            "app_secret":"secret"
        }
    }
```

`provider`为IM服务提供商，目前支持`rongcloud`表示融云IM。也可在进行本地开发调试时修改成`test`，调用IM服务时使用本地模拟的结果代替，而不实际请求IM服务。

`rongcloud`部分为融云IM服务的配置。`app_key`和`app_secret`表示融云的开发者app key和app secret，可以在融云的开发者后台页面中查看。

Prometheus监控配置

本服务支持暴露prometheus metrics HTTP接口，使用prometheus metrics格式提供各API的状态码计数、API耗时统计等监控信息，供prometheus拉取。同时，本服务支持将prometheus metrics格式的监控数据通过pushgateway推送到prometheus。
`prometheus`部分表示prometheus监控的配置。
```
    "prometheus":{
        "metrics_path":"/metrics",
        "enable_push":false # change this to true to enable pushing metrics to pushgateway
        "push_url": "127.0.0.1:9091",
        "push_interval_s":30
    }
```

`metrics_path` 表示提供prometheus metrics的路径，默认为`/metrics`。

`enable_push` 设置为 `true`时，将会定时把监控数据推送到pushgateway。`push_url`表示pushgateway的监听地址。`push_interval_s`表示推送监控数据的间隔时间，单位为秒。
