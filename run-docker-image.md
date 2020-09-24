## 互动直播后端镜像

### 构建镜像
```
QLIVE_IMAGE_TAG=latest make docker-image
```

### 运行镜像
需要依赖的数据库及外部服务包括：mongo数据库，七牛短信服务及RTC服务，融云IM服务。针对这些服务的配置需要在docker run命令中添加-e选项指定环境变量的方式注入到容器中。
```
docker run -e"<name>=<value>" -e"<name2>=<value2>" ... 
```
配置相关环境变量说明

|  名称                      |   含义                                 |
|  ----                      |  ----                                  |
|  QLIVE_HTTP_LISTEN_ADDR    |  HTTP 服务监听地址，不填默认:8080。       |
|  QLIVE_WS_LISTEN_ADDR      |  websocket 服务监听地址，不填默认:8082。    |
|  QLIVE_MONGODB_URI         |  mongo数据库地址，不填默认127.0.0.1:27017。 |
|  QLIVE_MONGODB_DATABASE    |  mongo数据库名称，不填默认qrtc_qlive。      |
|  QINIU_ACCESS_KEY          | 七牛密钥对中的 access key。             |
|  QINIU_SECRET_KEY         | 七牛密钥对中的 secret key。             |
|  QINIU_SMS_SIGN_ID        | 发送短信验证码使用的七牛云短信签名ID。     |
|  QINIU_SMS_TEMP_ID         |  七牛云短信模板ID。                      |
|  QINIU_RTC_APP_ID          |  七牛RTC 应用ID。                       |
|  QINIU_LIVE_PUBLISH_HOST   |  七牛直播推流目标域名。                   |
|  QINIU_LIVE_PUBLISH_HUB    |  七牛直播推流目标hub。                   |
|  RONGCLOUD_APP_KEY         |  融云应用 app key。                     |
|  RONGCLOUD_APP_SECRET      |  融云应用 app secret。                  |