# 地址解析 API — Linux 生产环境 Docker 部署指南

> 适用于 CentOS 7 / RHEL 7 服务器，通过 Docker Compose 部署 address-parse 服务。
>
> 服务器配置参考：28核 / 31GB 内存 / 1.7TB 磁盘

---

## 目录

- [1. 环境准备](#1-环境准备)
- [2. 上传代码](#2-上传代码)
- [3. 配置](#3-配置)
- [4. 启动](#4-启动)
- [5. 验证](#5-验证)
- [6. 常用运维命令](#6-常用运维命令)
- [7. 更新部署](#7-更新部署)
- [8. 卸载](#8-卸载)

---

## 1. 环境准备

### 1.1 检查 Docker 环境

```bash
docker --version
docker compose version
```

如果返回版本号 → 直接跳到 [2. 上传代码](#2-上传代码)

如果提示 `command not found` → 执行以下安装步骤

### 1.2 安装 Docker（CentOS 7）

```bash
# 升级 yum（非必须但推荐）
sudo yum update -y

# 安装依赖
sudo yum install -y yum-utils device-mapper-persistent-data lvm2

# 添加 Docker 官方仓库
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

# 安装 Docker Engine
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 启动并设置开机自启
sudo systemctl start docker
sudo systemctl enable docker

# 将当前用户加入 docker 组（避免每次 sudo）
sudo usermod -aG docker $USER
# 退出终端重新登录，使组权限生效
```

### 1.3 验证安装

```bash
docker run --rm hello-world
```

看到 `Hello from Docker!` 即成功。

---

## 2. 上传代码

### 2.1 在本机打包

在 **Mac/本地开发机** 上执行，将项目代码打包：

```bash
cd /Users/AI/ztd

# 排除不需要的文件，减少包体积
tar \
  --exclude='.git' \
  --exclude='.cursor' \
  --exclude='bin' \
  --exclude='*.test' \
  --exclude='coverage.html' \
  --exclude='coverage.out' \
  -czf address-parse.tar.gz .
```

### 2.2 上传到服务器

```bash
scp address-parse.tar.gz appuser@ztd-linux:/home/appuser/
```

> 如果提示 `scp: command not found`，在本机执行：
> ```bash
> brew install openssh
> ```

### 2.3 在服务器解压

```bash
# SSH 登录到服务器
ssh appuser@ztd-linux

cd ~
mkdir -p address-parse
cd address-parse
tar -xzf ../address-parse.tar.gz
ls -la
```

确认看到以下文件/目录：

```
Dockerfile         # 必须
docker-compose.yml # 必须
cmd/               # 必须
internal/          # 必须
pkg/               # 必须
migrations/        # 必须
.env               # 稍后创建
```

---

## 3. 配置

### 3.1 创建 .env 配置文件

```bash
cat > .env << 'ENVEOF'
# ============ 服务配置 ============
PORT=8080
MODE=release

# ============ 数据库（Docker Compose 模式下使用默认值） ============
DATABASE_URL=postgres://postgres:postgres@postgres:5432/address_parse

# ============ Redis（Docker Compose 模式下使用默认值） ============
REDIS_URL=redis://redis:6379

# ============ LLM 配置（必填） ============
# DashScope API Key，替换为你的真实密钥
DASHSCOPE_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxx
LLM_MODEL=qwen-turbo
LLM_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1

# ============ 认证配置（必填） ============
# 应用 ID，逗号分隔
APP_IDS=client_001,client_002
# 对应密钥，顺序与 APP_IDS 一致
APP_SECRETS=your-secret-key-1,your-secret-key-2

# ============ 限流配置 ============
# 全局限流（每分钟）
RATE_LIMIT_GLOBAL=5000
# 单应用限流（每分钟）
RATE_LIMIT_APP=500
# 单 IP 限流（每分钟）
RATE_LIMIT_IP=1000
ENVEOF
```

> **重要**：将 `DASHSCOPE_API_KEY` 替换为你的真实 DashScope 密钥。

### 3.2 配置说明

| 变量 | 说明 | 必填 |
|------|------|------|
| `PORT` | 服务端口，默认 8080 | 否 |
| `MODE` | `release` 表示生产模式 | 是 |
| `DATABASE_URL` | PostgreSQL 连接串，Docker 模式下用默认值 | 是 |
| `REDIS_URL` | Redis 连接串，Docker 模式下用默认值 | 是 |
| `DASHSCOPE_API_KEY` | 阿里云 DashScope API Key | **必填** |
| `APP_IDS` | 允许调用的客户端 ID | **必填** |
| `APP_SECRETS` | 对应密钥，与 ID 顺序一致 | **必填** |
| `RATE_LIMIT_GLOBAL` | 全局每分钟请求上限 | 否 |
| `RATE_LIMIT_APP` | 单应用每分钟上限 | 否 |
| `RATE_LIMIT_IP` | 单 IP 每分钟上限 | 否 |

---

## 4. 启动

### 4.1 首次启动

```bash
cd ~/address-parse

# 构建镜像并启动所有容器（app + postgres + redis）
docker compose up -d --build
```

这一步会：
1. 下载 `golang:1.21-alpine` 构建镜像
2. 下载 `postgres:16-alpine` 数据容器
3. 下载 `redis:7-alpine` 缓存容器
4. 编译 Go 程序
5. 启动三个容器并连接网络

首次启动大约需要 3-5 分钟（取决于网络），之后启动约 30 秒。

### 4.2 检查容器状态

```bash
docker compose ps
```

正常输出示例：

```
NAME                        IMAGE                  STATUS
address-parse-app-1         address-parse-app     Up (healthy)
address-parse-postgres-1    postgres:16-alpine    Up (healthy)
address-parse-redis-1       redis:7-alpine        Up
```

> `app-1` 显示 `(healthy)` 才算真正就绪。

---

## 5. 验证

### 5.1 健康检查

```bash
curl http://localhost:8080/health
```

期望返回：

```json
{"code":0,"message":"success","data":{"status":"ok"}}
```

### 5.2 API 调用测试

```bash
# 生成签名并调用解析接口
APP_SECRET="your-secret-key-1"
BODY='{"name":"张三","phone":"15361237638","company":"智腾达","address":"广东省深圳市南山区桃源街道88号"}'
TS=$(date +%s)
SIG=$(echo -n "${TS}${BODY}" | openssl dgst -sha256 -hmac "$APP_SECRET" -binary | base64)

curl -s -X POST "http://localhost:8080/api/v1/address/parse" \
  -H "Content-Type: application/json" \
  -H "X-App-Id: client_001" \
  -H "X-Timestamp: ${TS}" \
  -H "X-Signature: ${SIG}" \
  -d "${BODY}" | python3 -m json.tool
```

期望返回：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "name": "张三",
        "phone": "15361237638",
        "company": "智腾达",
        "province": "广东省",
        "city": "深圳市",
        "district": "南山区",
        "street": "桃源街道",
        "detail": "88号",
        "full_address": "广东省 深圳市 南山区 桃源街道 88号"
    }
}
```

---

## 6. 常用运维命令

### 查看日志

```bash
# 实时查看 app 日志（按 Ctrl+C 退出）
docker compose logs -f app

# 查看最近 100 行
docker compose logs --tail=100 app

# 查看所有容器日志
docker compose logs -f
```

### 启停服务

```bash
# 停止（数据保留在 volume 中）
docker compose stop

# 启动
docker compose start

# 重启
docker compose restart

# 停止并删除容器（volume 不删除，数据保留）
docker compose down

# 停止并删除容器 + volume（数据彻底清除）
docker compose down -v
```

### 进入容器调试

```bash
# 进入 app 容器
docker compose exec app sh

# 进入 postgres
docker compose exec postgres psql -U postgres -d address_parse

# 进入 redis
docker compose exec redis redis-cli
```

### 查看资源占用

```bash
docker stats
```

---

## 7. 更新部署

### 7.1 方式一：重新打包上传（本项目适用）

```bash
# 1. 在本机重新打包
cd /Users/AI/ztd
tar \
  --exclude='.git' \
  --exclude='.cursor' \
  --exclude='bin' \
  -czf address-parse.tar.gz .

# 2. 上传
scp address-parse.tar.gz appuser@ztd-linux:/home/appuser/

# 3. 在服务器执行更新
ssh appuser@ztd-linux
cd ~/address-parse

# 备份旧配置
cp .env .env.bak

# 解压覆盖
tar -xzf ../address-parse.tar.gz --overwrite

# 恢复配置（避免被覆盖）
cp .env.bak .env

# 重建并重启
docker compose up -d --build
```

### 7.2 方式二：只更新代码目录（不传 .tar.gz）

如果只改了少量代码，可以让服务器从 git pull 更新（如果有 git 仓库）：

```bash
cd ~/address-parse
git pull
docker compose up -d --build
```

### 7.3 金丝雀发布（推荐生产使用）

```bash
# 先保留旧容器，启动新版本验证
docker compose up -d --build

# 确认新版本正常后，停止旧容器
docker compose stop app_old
docker compose rm -f app_old

# 如果新版本有问题，回滚
docker compose down
docker compose -f docker-compose.yml down  # 删除新版本
# 重新启动旧版本
docker compose up -d
```

---

## 8. 卸载

```bash
# 停止所有容器并删除
cd ~/address-parse
docker compose down -v

# 删除代码目录
cd ~
rm -rf ~/address-parse

# 删除 Docker（可选，如需完全清理）
sudo systemctl stop docker
sudo systemctl disable docker
sudo yum remove -y docker-ce docker-ce-cli containerd.io
sudo rm -rf /var/lib/docker
```

---

## 附录：防火墙配置

如果服务器开启了 firewalld，需要开放端口：

```bash
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload

# 验证
sudo firewall-cmd --list-ports
```

如果是云服务器（阿里云/腾讯云），还需在云控制台安全组中放行 8080 端口。

---

## 故障排查

### 容器启动失败

```bash
# 查看详细日志
docker compose logs app

# 常见原因：
# 1. .env 中 DASHSCOPE_API_KEY 为空 → 必填字段
# 2. 端口 8080 被占用 → 改端口或 kill 占用进程
```

### 健康检查不通过

```bash
# 检查依赖容器是否就绪
docker compose ps

# postgres 未就绪时 app 会等待，耐心等待 30s 再试
# 查看 postgres 日志
docker compose logs postgres
```

### API 返回 429 Rate Limit

正常现象，说明限流在正常工作。如果频繁出现，需要调高 `.env` 中的限流阈值：

```bash
# 编辑 .env
nano .env
# 修改 RATE_LIMIT_GLOBAL / RATE_LIMIT_APP / RATE_LIMIT_IP

# 重启生效
docker compose restart app
```

### 内存不足

如果 `docker stats` 显示 app 内存接近上限，编辑 `docker-compose.yml` 调高限制：

```yaml
deploy:
  resources:
    limits:
      cpus: "2.0"        # 从 1.0 调高
      memory: 1024M       # 从 512M 调高
```
