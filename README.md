# Caddy 可信代理模块 - 腾讯云边缘安全加速平台 EO 集成

本模块通过腾讯云边缘安全加速平台 EO (EdgeOne) API 自动获取回源 IP 网段，并将其配置为 Caddy 的可信代理。该模块调用腾讯云的 [查询源站防护详情](https://cloud.tencent.com/document/api/1552/120408) API 来动态获取和更新应当被信任的代理 IP 地址列表。

此模块适用于腾讯云国内站。

## 功能特性

- 自动从腾讯云边缘安全加速平台 EO 获取回源 IP 网段
- 同时支持 IPv4 和 IPv6 地址
- 可配置的刷新间隔
- 可配置的请求超时时间
- 支持环境变量替换凭证信息

## 安装方式

### 使用 Docker

推荐使用预构建的 Docker 镜像：

```bash
docker pull ghcr.io/mnixry/caddy-edgeone-ip:edge
```

### 源码编译

使用此模块构建 Caddy：

```bash
xcaddy build --with github.com/mnixry/caddy-edgeone-ip
```

## 配置说明

### 基础配置

在您的 Caddyfile 中的 `trusted_proxies` 指令下添加以下配置：

```Caddyfile
# 大括号里面是可选参数
trusted_proxies edgeone <zone_id> <secret_id> <secret_key> {
    interval 12h
    timeout 15s
    api_endpoint teo.tencentcloudapi.com
}
```

### 使用环境变量示例

为了安全起见，建议使用环境变量存储凭证：

```bash
export EONE_ZONE_ID="your-zone-id"
export EONE_SECRET_ID="your-secret-id"
export EONE_SECRET_KEY="your-secret-key"
```

然后在 Caddyfile 中：

```Caddyfile
trusted_proxies edgeone {$EONE_ZONE_ID} {$EONE_SECRET_ID} {$EONE_SECRET_KEY} {
    interval 1h
    timeout 30s
}
```

### 完整配置示例

```Caddyfile
{
    trusted_proxies edgeone zone-abc123 LTAI4G... your-secret-key {
        interval 2h
        timeout 10s
        api_endpoint teo.tencentcloudapi.com
    }
}

example.com {
    reverse_proxy localhost:8080
}
```

## 配置参数

| 参数名称     | 描述                              | 类型     | 默认值                  | 是否必需 |
| ------------ | --------------------------------- | -------- | ----------------------- | -------- |
| zone_id      | 腾讯云边缘安全加速平台 EO 站点 ID | string   | -                       | 是       |
| secret_id    | 腾讯云 API 密钥 ID                | string   | -                       | 是       |
| secret_key   | 腾讯云 API 密钥 Key               | string   | -                       | 是       |
| interval     | 获取 EO IP 网段列表的刷新间隔     | duration | 1h                      | 否       |
| timeout      | API 请求的最大等待时间            | duration | 无超时                  | 否       |
| api_endpoint | 腾讯云 API 接口地址               | string   | teo.tencentcloudapi.com | 否       |
