# qualvey/pkg

Go 后端微核心基础设施库 (Universal Micro-Core Infrastructure Library for Go)。

专为单人全栈、独立产品与高可靠微服务设计的开箱即用通用基建组件库。零多余第三方依赖，纯 Go 标准库打造，跨项目一键即插即用。

---

## 模块列表 (Modules)

| 模块 | 说明 | 状态 |
| :--- | :--- | :--- |
| **[`logger`](./logger)** | 基于 `log/slog` 的高性能双模自适应日志系统（终端彩色 Pretty + 生产单行 JSON + Context 链路透传 + HTTP 中间件 + 启动看板） | ✅ 稳定可用 (v0.1.0) |
| `response` | 统一 RESTful API 响应与业务错误码体系 | 规划中 |
| `env` | 环境变量类型安全读取与生产必填项强校验 | 规划中 |
| `auth` | 纯标准库 JWT 签发、校验与 Context 透传门面 | 规划中 |
| `database` | SQLite WAL / PG 驱动连接池配置与自动事务闭包 | 规划中 |
| `middleware` | RequestID / CORS / Panic 恢复 / 防刷限流套件 | 规划中 |

---

## 安装与引入 (Installation)

```bash
go get github.com/qualvey/pkg@latest
```

---

## 快速上手：`logger` 模块

### 1. 核心设计特性
- **双模智能自适应**：无缝支持历史 `printf` 格式化调用（`"%s %d"`），也支持标准 `slog` 结构化键值对（`"user", uid`），绝不破坏老项目现有业务代码。
- **开箱即用高亮**：本地开发与控制台默认输出带 ANSI 终端颜色的 `PrettyHandler`；生产环境自动切换符合云原生/Loki 标准的单行 `json`。
- **全链路追踪**：内置 `HTTPMiddleware`，自动提取/生成 `X-Request-ID`，并将携带追踪属性的子 Logger 注入请求 `r.Context()`。
- **启动自检看板**：内置 `PrintBootSummary`，服务监听前格式化输出版本、环境、安全凭证掩码与数据库状态。

### 2. 基础调用示例

```go
package main

import (
	"context"

	"github.com/qualvey/pkg/logger"
)

func main() {
	// 1. 初始化（环境、级别、格式）
	// 支持: logger.Init("development", "DEBUG", "pretty")
	// 或空参自动自适应: logger.Init("", "", "")
	logger.Init("", "", "")

	// 2. 传统 printf 格式化调用（自动识别格式化动词）
	logger.Info("🚀 服务正在启动，端口: %d, 环境: %s", 8080, "dev")

	// 3. 结构化 Key-Value 属性调用（Slog 原生特性）
	logger.Info("HTTP request processed",
		"status", 200,
		"dur_ms", 1.45,
		"user_id", "u_1001",
	)

	// 4. Context 链路追踪透传
	ctx := logger.WithAttrs(context.Background(), "trace_id", "tr_998877")
	logger.InfoContext(ctx, "订单支付成功", "order_no", "ORD_20261111")
}
```

---

## 跨项目本地免发布联调 (`go.work`)

在你的多项目代码根目录下配置工作区，无需反复 push 或发 tag：

```bash
go work init ./your-project ./pkg
```

本地修改 `pkg` 源码，业务工程秒级热生效。

---

## License

MIT License
