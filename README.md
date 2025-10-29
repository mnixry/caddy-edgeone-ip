# Caddy 可信代理模块 - 腾讯云边缘安全加速平台 EO 集成

本模块通过腾讯云边缘安全加速平台 EO (EdgeOne) API 实时验证请求来源 IP 是否为 EdgeOne 节点，并将验证通过的 IP 配置为 Caddy 的可信代理。该模块调用腾讯云的 [查询 IP 归属信息](https://cloud.tencent.com/document/api/1552/102227) API 来验证每个请求的来源 IP 地址。

此模块适用于腾讯云国内站。

## 功能特性

- 实时验证请求来源 IP 是否为 EdgeOne 节点
- 同时支持 IPv4 和 IPv6 地址
- 内置 LRU 缓存机制，提高验证性能
- 可配置的缓存大小和过期时间
- 可配置的 API 请求超时时间
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
trusted_proxies edgeone <secret_id> <secret_key> {
    cache_ttl 1h
    cache_size 1000
    timeout 5s
    api_endpoint teo.tencentcloudapi.com
}
```

### 使用环境变量示例

为了安全起见，建议使用环境变量存储凭证：

```bash
export EONE_SECRET_ID="your-secret-id"
export EONE_SECRET_KEY="your-secret-key"
```

然后在 Caddyfile 中：

```Caddyfile
trusted_proxies edgeone {$EONE_SECRET_ID} {$EONE_SECRET_KEY} {
    cache_ttl 1h
    cache_size 500
    timeout 5s
}
```

### 完整配置示例

```Caddyfile
{
    trusted_proxies edgeone LTAI4G... your-secret-key {
        cache_ttl 2h
        cache_size 2000
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
| secret_id    | 腾讯云 API 密钥 ID                | string   | -                       | 是       |
| secret_key   | 腾讯云 API 密钥 Key               | string   | -                       | 是       |
| cache_ttl    | IP 验证结果的缓存过期时间         | duration | 1h                      | 否       |
| cache_size   | LRU 缓存的最大条目数量            | int      | 1000                    | 否       |
| timeout      | API 请求的最大等待时间            | duration | 5s                      | 否       |
| api_endpoint | 腾讯云 API 接口地址               | string   | teo.tencentcloudapi.com | 否       |
