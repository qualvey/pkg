# qualvey/pkg 项目规范与基建模块设计指南 (Architecture & Implementation Guidelines)

> 本文档旨在确立 `qualvey/pkg` 基础设施库的设计哲学、编码规范，以及各个子模块的定位、技术选型与具体实现方案。
> 目的是在后续启动类似后端业务工程时，能够以极少样板代码（Boilerplate）在分钟级快速搭建起高可靠、可观测、标准化的 Go 后端服务。

---

## 目录 (Table of Contents)

1. [项目愿景与核心设计哲学 (Core Philosophy)](#1-项目愿景与核心设计哲学-core-philosophy)
2. [公共库设计与工程规范 (Engineering Standards)](#2-公共库设计与工程规范-engineering-standards)
3. [模块详细定位与实现蓝图 (Module Blueprints)](#3-模块详细定位与实现蓝图-module-blueprints)
   - [3.1 logger：自适应结构化日志与可观测性链路 (已就绪)](#31-logger自适应结构化日志与可观测性链路-已就绪)
   - [3.2 env：类型安全环境变量读取与生产校验](#32-env类型安全环境变量读取与生产校验)
   - [3.3 response：统一 RESTful 响应与业务错误码体系](#33-response统一-restful-响应与业务错误码体系)
   - [3.4 auth：标准 JWT 门面与 Context 透传](#34-auth标准-jwt-门面与-context-透传)
   - [3.5 database：连接池调优配置与安全事务闭包](#35-database连接池调优配置与安全事务闭包)
   - [3.6 middleware：通用 HTTP 请求治理中间件套件](#36-middleware通用-http-请求治理中间件套件)
4. [新服务 10 行启动样板 (Quick Bootstrap Pattern)](#4-新服务-10-行启动样板-quick-bootstrap-pattern)
5. [跨工程本地开发协同规范 (`go.work`)](#5-跨工程本地开发协同规范-gowork)

---

## 1. 项目愿景与核心设计哲学 (Core Philosophy)

### 1.1 痛点背景
在单人全栈或敏捷微服务开发中，每次新建 Go 工程往往都要反复编写：
- 日志配置、终端着色与 JSON 格式判断
- 繁杂的 `os.Getenv` 与类型转换、生产环境缺失字段校验
- HTTP 统一 JSON 包装、错误码定义与错误转译
- JWT 签发、校验与 Context 存取
- SQLite WAL / PG 连接池参数微调与防死锁配置
- CORS、RequestID 传递、Panic 恢复防崩溃

大量业务无关的**胶水代码**不仅浪费开发时间，而且在不同项目间拷贝容易产生细微实现差异，导致潜在 Bug（例如 SQLite 锁库、JWT 算法混淆漏洞、Panic 导致进程直接退出等）。

### 1.2 三大核心设计原则
1. **零冗余依赖，恪守标准库优先（Standard Library First）**：
   - 坚决避免为了简单功能引入重型第三方库（如拒绝为了读配置引入庞大的 Viper）。
   - 核心层一律基于 Go 标准库（`log/slog`、`net/http`、`context`、`crypto`、`database/sql`）设计，对上层 Web 框架（如 Gin）只做非侵入式的“轻薄适配器（Adapter）”。
2. **拒绝盲目重复造轮子（No Reinventing Vulnerable Wheels）**：
   - **涉及算法安全、密码学、深层 RFC 的领域绝不手写底层**（例如 JWT 规范严格复用行业标准的 `golang-jwt/jwt/v5`，仅在其上封装易用的 Context 门面，避免时序攻击和算法漏洞）。
   - **涉及项目规范与胶水治理的轻量代码坚决自研**（如统一 Response、配置校验、中间件链路）。
3. **安全自检与开箱即用（Fail-Fast & Zero-Config）**：
   - 默认参数必须符合生产高可用要求（如默认开启 SQLite WAL 与超时、PG 连接池生命周期控制）。
   - 启动期具备环境自检能力（生产环境必填项缺失立即 Panic 拦截，阻断带病上线）。

---

## 2. 公共库设计与工程规范 (Engineering Standards)

### 2.1 包与目录组织
- 每个公共模块独立成顶层 package（如 `pkg/logger`、`pkg/env`、`pkg/response`）。
- 模块间单向依赖，杜绝循环引用。底层模块（`logger`, `env`）不可反向依赖高层模块（`middleware`, `database`）。
- 内部复杂逻辑置于包内未导出文件中，对外仅暴露干净语义化 API。

### 2.2 错误处理体系
- **区分系统底层错误与业务错误**：
  - 底层系统错误使用标准库 `fmt.Errorf("...: %w", err)` 保持错误链。
  - 面向 API 交互的错误定义为 `BizError`，携带 HTTP 状态码、业务错误码与展示文本。
- **Panic 处理原则**：公共库函数除 `Init`/`Load` 阶段在生产必填配置缺失时允许 `panic` 快速拦截外，所有运行时函数**一律返回 error，严禁在运行时内部 panic**。

### 2.3 Context 透传约定
- 所有包含生命周期、请求追踪、用户身份的操作必须以 `ctx context.Context` 作为第一入参。
- 采用私有空结构体类型定义 context key，防止 key 冲突：
  ```go
  type requestIDKey struct{}
  type authClaimsKey struct{}
  ```

---

## 3. 模块详细定位与实现蓝图 (Module Blueprints)

### 3.1 logger：自适应结构化日志与可观测性链路 (已就绪)
* **定位**：基于 `log/slog` 的高性能双模自适应日志底座，全链路日志规范基石。
* **核心能力**：
  - 开发环境终端彩色 Pretty 输出；生产环境自动切换符合云原生/Loki 标准的单行 JSON。
  - 双模智能识别：无缝支持历史 `printf` 格式化调用（`"%s %d"`）与 `slog` 结构化键值对（`"user", uid`）。
  - 内置 `HTTPMiddleware`，自动提取/生成 `X-Request-ID` 并透传至 Context。
  - 内置 `PrintBootSummary` 启动看板，打印版本、掩码配置与自检摘要。

---

### 3.2 env：类型安全环境变量读取与生产校验
* **定位**：声明式结构体驱动的配置安全加载器，替代重量级 Viper 与易遗漏的手写 `os.Getenv`。
* **选型策略**：**纯标准库自研 (零外部依赖)**。使用 `os` + `reflect` + `strconv` + `strings`。
* **功能清单**：
  1. 结构体 Tag 驱动：`env:"KEY"`、`default:"value"`、`required:"prod"`。
  2. 纯标准库极简 `.env` 文件读取（支持 `#` 注释与基础引用，取代 `godotenv`）。
  3. 类型自动转换：`string`、`int/int64`、`bool`（智能识别 `true/1/yes/false/0/no`）、`time.Duration`、`[]string`（逗号分隔切片）。
  4. 生产禁忌自检：支持 `no_localhost:"true"`，防止在生产环境误将回调或数据库地址配为本地开发端口。
* **对外 API 示范**：
  ```go
  type AppConfig struct {
      Port        int           `env:"PORT" default:"8080"`
      Env         string        `env:"APP_ENV" default:"development"`
      JWTSecret   string        `env:"JWT_SECRET" required:"prod"`
      BackendURL  string        `env:"BACKEND_URL" required:"prod" no_localhost:"prod"`
      AllowedHost []string      `env:"ALLOWED_HOSTS" default:"*"`
      Timeout     time.Duration `env:"HTTP_TIMEOUT" default:"10s"`
  }

  // Load 会自动尝试加载本地 .env，并将环境变量解析映射进 cfg 结构体，验证不通过时返回 error
  func Load(cfg any) error
  ```

---

### 3.3 response：统一 RESTful 响应与业务错误码体系
* **定位**：规范前后端交互协议契约，提供类型安全的错误码体系与标准 JSON 输出。
* **选型策略**：**自研核心 + 纯标准库契约 + Gin 适配器**。
* **功能清单**：
  1. 标准统一响应结构：
     ```json
     {
       "code": 200,
       "message": "success",
       "data": { ... },
       "error": "OPTIONAL_ERROR_TYPE",
       "trace_id": "req-xxxx-yyyy"
     }
     ```
  2. 统一分页数据包装 `PageData[T any]`，统一 `list`, `total`, `page`, `pageSize` 命名。
  3. `BizError` 抽象接口与常用预定义错误（`ErrBadRequest`、`ErrUnauthorized`、`ErrForbidden`、`ErrNotFound`、`ErrInternal`）。
  4. 解耦设计：底层面向标准库 `http.ResponseWriter`，同时提供面向 Gin 的薄封装（`response.Success(c, data)` / `response.Fail(c, bizErr)`）。
* **对外 API 示范**：
  ```go
  // 标准库入口
  func JSON(w http.ResponseWriter, httpStatus int, code int, msg string, data any)
  func Error(w http.ResponseWriter, r *http.Request, err error)

  // Gin 门面扩展
  func Success(c *gin.Context, data any)
  func SuccessPage[T any](c *gin.Context, list []T, total int64)
  func Fail(c *gin.Context, err error)
  ```

---

### 3.4 auth：标准 JWT 门面与 Context 透传
* **定位**：开箱即用的轻量级鉴权与身份识别门面。
* **选型策略**：**依赖社区事实标准 `golang-jwt/jwt/v5` 作为底层引擎，上层封装纯粹门面**。（坚决不自己从零手写加密签名校验，避免安全漏洞）。
* **功能清单**：
  1. 泛型 Payload 机制：`Claims[T any]`，允许业务工程自由定义其专属的用户信息实体，无需在底层将字段强行写死为 `Role` 或 `ShopName`。
  2. 极简签发与校验器：`Manager[T any]` 提供 `Sign(data T, ttl time.Duration)` 与 `Verify(tokenStr string)`。
  3. Context 注入与提取门面：`auth.WithUser(ctx, user)` / `auth.FromContext[T](ctx)`。
  4. 标准 HTTP 与 Gin 鉴权中间件生成器，自动解析 `Authorization: Bearer <token>`。
* **对外 API 示范**：
  ```go
  type Manager[T any] struct { ... }
  func New[T any](secret string, defaultTTL time.Duration) *Manager[T]
  func (m *Manager[T]) Sign(user T) (string, error)
  func (m *Manager[T]) Parse(tokenString string) (T, error)

  // 中间件与 Context
  func (m *Manager[T]) HTTPMiddleware(optional bool) func(http.Handler) http.Handler
  func (m *Manager[T]) GinMiddleware(optional bool) gin.HandlerFunc
  func CurrentUser[T any](ctx context.Context) (T, bool)
  ```

---

### 3.5 database：连接池调优配置与安全事务闭包
* **定位**：数据库生产级参数沉淀集与无泄露事务封装。
* **选型策略**：**不造通用 ORM 轮子；提供 SQLite/PG 标准连接池调优模板与 Panic-Safe 事务闭包**。
* **功能清单**：
  1. **SQLite 生产化预设（防锁与并发保障）**：
     - 自动开启 `PRAGMA journal_mode = WAL;`
     - 自动配置 `PRAGMA busy_timeout = 5000;`
     - 强制限制单一写入连接 `SetMaxOpenConns(1)`，防止 SQLite 出现 `database is locked` 报错。
  2. **PostgreSQL 连接池预设**：
     - 调优参数标准化：`MaxOpenConns`、`MaxIdleConns`、`ConnMaxLifetime`、`ConnMaxIdleTime`。
  3. **安全事务闭包 (Safe Transaction Closure)**：
     - 支持标准库 `*sql.DB` 及 GORM。
     - **内置 Panic 拦截器**：若闭包内发生 `panic`，先执行 Rollback 释放数据库连接与锁，再向外传递 panic，彻底解决连接池泄漏问题。
* **对外 API 示范**：
  ```go
  type PoolConfig struct {
      MaxOpenConns    int
      MaxIdleConns    int
      ConnMaxLifetime time.Duration
  }
  func TunePostgres(db *sql.DB, cfg PoolConfig)
  func TuneSQLiteWAL(db *sql.DB, busyTimeout time.Duration) error

  // 标准库事务安全闭包
  func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error
  ```

---

### 3.6 middleware：通用 HTTP 请求治理中间件套件
* **定位**：通用微核心中间件套件，涵盖链路跟踪、安全恢复、跨域与限流防护。
* **选型策略**：**基于标准库 `http.Handler` 编写，无缝打通 `pkg/logger`，提供 Gin 桥接函数**。
* **功能清单**：
  1. **RequestID**：读取或生成 UUID/NanoID，回写响应头 `X-Request-ID`，并将带有 `request_id` 属性的子 Logger 存入请求 `r.Context()`。
  2. **Recovery**：捕获未处理的运行时 panic，使用 `pkg/logger` 结构化输出崩溃堆栈，向客户端响应标准 500 JSON，防止进程闪退。
  3. **CORS**：纯标准库实现现代跨域头配置，支持安全匹配开发本地源与生产限定域名，无需额外引入外部库。
  4. **RateLimit（防刷限流）**：
     - 抽象通用的 `Limiter` 接口。
     - 默认内置纯标准库实现的**并发安全内存令牌桶**（适用于单机部署或无 Redis 场景）。
     - 提供 Redis 滑动窗口可选驱动（降级容灾：Redis 故障时自动无缝退化为内存限流）。
* **对外 API 示范**：
  ```go
  // 标准中间件 (同时提供 ToGin(...) 转换适配器)
  func RequestID() func(http.Handler) http.Handler
  func Recovery() func(http.Handler) http.Handler
  func CORS(opts CORSOptions) func(http.Handler) http.Handler
  func RateLimit(limiter Limiter) func(http.Handler) http.Handler
  ```

---

## 4. 新服务 10 行启动样板 (Quick Bootstrap Pattern)

后续新服务起步时，只需以下极度简练的编排代码，即可拥有完备的基础设施支持：

```go
package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qualvey/pkg/auth"
	"github.com/qualvey/pkg/env"
	"github.com/qualvey/pkg/logger"
	"github.com/qualvey/pkg/middleware"
	"github.com/qualvey/pkg/response"
)

type Config struct {
	Port      int    `env:"PORT" default:"8080"`
	Env       string `env:"APP_ENV" default:"dev"`
	JWTSecret string `env:"JWT_SECRET" required:"prod"`
}

type UserSession struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
}

func main() {
	// 1. 类型安全加载环境变量 (生产环境必填强校验)
	var cfg Config
	if err := env.Load(&cfg); err != nil {
		panic(err)
	}

	// 2. 初始化结构化日志 (开发终端彩色 / 生产单行 JSON)
	logger.Init(cfg.Env, "INFO", "")
	logger.PrintBootSummary("MyNewService", "1.0.0", cfg.Env, map[string]string{
		"Port": "8080",
	})

	// 3. 初始化鉴权管理器
	authMgr := auth.New[UserSession](cfg.JWTSecret, 24*time.Hour)

	// 4. 组装路由与微核心中间件
	r := gin.New()
	r.Use(
		middleware.GinRecovery(),
		middleware.GinRequestID(),
		middleware.GinCORS(middleware.DefaultCORSConfig()),
	)

	// 5. 业务路由编写
	r.GET("/api/v1/ping", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	r.GET("/api/v1/profile", authMgr.GinMiddleware(false), func(c *gin.Context) {
		user, _ := auth.CurrentUser[UserSession](c.Request.Context())
		response.Success(c, user)
	})

	r.Run(":8080")
}
```

---

## 5. 跨工程本地开发协同规范 (`go.work`)

`qualvey/pkg` 采用 Go 1.18+ 的 Multi-Module Workspaces 特性进行日常开发。在项目群的统一上级目录下：

```bash
# 1. 建立工作区并将 pkg 与具体业务工程同时加入
go work init ./pkg ./Mall/backend ./other-service

# 2. 本地开发与联调
# 在本地直接修改 pkg 下的代码，业务服务无需重新 go get 或打 tag，立即全局生效调试。

# 3. 提交与发版
# 确保各个模块完成单元测试 (go test -v -race ./...) 后，推送并在 pkg 仓库打语义化 tag (如 v0.2.0)。
```
