# order-service 订单中心

订单中心服务：负责**订单创建 / 查询 / 取消**、**支付单创建**、**支付回调**、**订单超时取消**，
并通过 Dubbo 调用 `product-service` 完成商品校验与库存预占/确认/释放。

商品与库存的数据所有权仍在 `product-service`，本服务只保存订单相关的四张表。

## 1. 在整体架构中的位置

```
                    gateway (8000)
                       │  /api/v1/orders/**  /api/v1/payments/**
                       ↓
                order-service (8004)
                       │
          ┌────────────┼────────────┐
          ↓            ↓            ↓
       Order        Payment      Timeout(定时任务)
          │            │
          └──────┬─────┘
                 ↓
          MySQL(order_db)
                 │ Dubbo 直连/Nacos
                 ↓
        product-service (8003) → Product / SKU / Inventory
```

| 服务 | 端口 | 说明 |
| --- | --- | --- |
| gateway-service | 8000 | JWT 校验 + 注入 `uid` / `X-User-Id` / `X-User-Role` |
| user-service | 8001 | |
| auth-service | 8002 | |
| product-service | 8003 | Dubbo provider，暴露 `ProductRpcService` / `StockRpcService`（20880） |
| **order-service** | **8004** | 消费方 + HTTP 服务 |

## 2. 技术栈

Spring Boot 3.3.5 / Java 22 / MyBatis（手写 XML，无 MyBatis-Plus）/ MySQL 8 / Dubbo 3.3.0 / Nacos。
测试用 H2（MODE=MySQL）+ MockMvc + `@MockBean`。

## 3. 目录结构

```
src/main/java/com/example/order
├── OrderServiceApplication.java        @SpringBootApplication @MapperScan @EnableScheduling
├── api/                                ← product-service 的 RPC「契约副本」（消费方必须持有）
│   ├── ProductRpcService.java  StockRpcService.java
│   └── dto/  ProductDTO  SkuDTO  StockDTO
├── common/                             统一响应 / 错误码 / 业务异常 / ID 生成 / 当前用户 / 分页
│   └── http/                           X-Request-Id 链路过滤器
├── config/                             Jackson 时间格式 / 配置项 / Dubbo
├── controller/                         OrderController  PaymentController
├── domain/
│   ├── entity/                         Order  OrderItem  PaymentOrder  PaymentRecord
│   └── enums/                          OrderStatus  PaymentStatus  PaymentChannel  CallbackStatus
│                                       SkuStatus / ProductStatus（镜像 product-service 的状态码）
├── dto/request|response/               CreateOrderRequest  CreatePaymentRequest  PaymentCallbackRequest
│                                       OrderVO  OrderItemVO  PaymentVO
├── mapper/ + resources/mapper/*.xml    四张表的 Mapper 与 SQL
├── service/                            
│   └── impl/                           OrderServiceImpl  PaymentServiceImpl
├── support/                            OrderLookup  ProductRpcClient  PaymentSigner  PaymentRecordWriter
└── task/                               OrderTimeoutTask（超时取消 + 权益发放补偿）
```

## 4. HTTP 接口

所有接口统一返回 `ApiResponse`：`{"code": 0, "message": "success", "data": ...}`。
除支付回调外都要求登录 —— 身份来自网关注入的 `uid` 头，**服务端不接受请求体里的用户标识**。

| 方法 | 路径 | 用途 | 登录 |
| --- | --- | --- | --- |
| POST | `/api/v1/orders` | 创建订单 | ✅ |
| GET | `/api/v1/orders/{id}` | 查询订单详情 | ✅ |
| GET | `/api/v1/orders?status=&page=&size=` | 查询我的订单（分页） | ✅ |
| POST | `/api/v1/orders/{id}/cancel` | 取消订单 | ✅ |
| POST | `/api/v1/orders/{id}/pay` | 创建支付单 | ✅ |
| POST | `/api/v1/payments/callback` | 支付回调（验签，免登录） | ❌ |

> `{id}` 支持两种形态：订单业务编号（`O...`）或内部数字主键。

### 4.1 创建订单

```http
POST /api/v1/orders
uid: u-1001
Content-Type: application/json

{
  "items": [
    {"skuId": "S1ABC...", "quantity": 2},
    {"skuId": "S2DEF...", "quantity": 1}
  ]
}
```

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 12,
    "orderId": "O2AB7K9Q3X1M",
    "uid": "u-1001",
    "status": 0,
    "statusName": "待支付",
    "totalAmount": 20800,
    "expireAt": "2026-09-22 18:08:05",
    "items": [
      {"id": 21, "productId": "P1", "skuId": "S1ABC...", "productName": "商品A",
       "skuName": "规格-默认", "unitPrice": 9900, "quantity": 2, "totalAmount": 19800}
    ]
  }
}
```

> **价格只信服务端**：`unitPrice` 由 RPC 实时读取 SKU 售价后写入 `order_item` 快照，
> 请求体里即使传了价格也会被忽略；下单后商品改名改价不影响历史订单。
> 金额单位为**分**，用 `long` 计算并做溢出保护。

### 4.2 创建支付单

```http
POST /api/v1/orders/O2AB7K9Q3X1M/pay
uid: u-1001

{"paymentChannel": 1}      // 1-微信 2-支付宝 3-余额
```

金额取自订单 `total_amount`。**幂等**：订单已有未支付的支付单时直接复用，不会重复创建。

### 4.3 支付回调（渠道 → 服务）

```http
POST /api/v1/payments/callback
Content-Type: application/json

{
  "paymentId": "P2AB7K9Q4Y2N",
  "transactionId": "WX-TX-0001",
  "paymentChannel": 1,
  "tradeStatus": "SUCCESS",
  "amount": 20800,
  "sign": "3f2a...（HmacSHA256 十六进制小写）"
}
```

验签串（按字段名升序拼接）：

```
amount={amount}&paymentChannel={paymentChannel}&paymentId={paymentId}&tradeStatus={tradeStatus}&transactionId={transactionId}
```

本地构造合法回调（Java）：注入 `PaymentSigner` 调 `sign(new PaymentCallbackRequest(...))` 即可；
密钥由 `order.payment.callback-secret` 配置，未配置时会**跳过验签并打 WARN**（仅限本地联调）。

### 4.4 错误码

| code | HTTP | 含义 |
| --- | --- | --- |
| 40001 | 400 | 请求参数不合法（含渠道非法、重复 SKU、请求体格式错误） |
| 40002 | 400 | 支付回调签名校验失败 |
| 40101 | 401 | 未登录 / 缺少网关身份头 |
| 40401 | 404 | 订单不存在（**他人订单一律按不存在处理**，不泄露资源存在性） |
| 40402 | 404 | 支付单不存在 |
| 40403 / 40404 | 404 | 商品不存在 / SKU 不存在（RPC 返回 null） |
| 40901 | 409 | 订单状态不允许该操作 |
| 40902 / 40903 | 409 | 商品未上架 / SKU 不可售 |
| 40904 | 409 | 可用库存不足 |
| 40905 | 409 | 订单已超时 |
| 40906 | 409 | 回调金额与支付单金额不一致 |
| 40908 | 409 | 渠道流水号已被其它支付单占用 |
| 40909 | 409 | 唯一索引冲突（并发重复提交兜底） |
| 50000 | 500 | 服务器内部错误 |
| 50001 | 503 | 远程服务（product-service）调用失败 |

## 5. 订单状态机

```
WAIT_PAY ──支付成功──► PAID ──权益发放完成──► COMPLETED
    │
    └────用户取消 / 超时────► CANCELLED
```

| 状态 | code | 说明 |
| --- | --- | --- |
| `WAIT_PAY` | 0 | 待支付，可取消、可创建支付单 |
| `PAID` | 1 | 已支付，等待权益发放（库存确认） |
| `COMPLETED` | 2 | 权益发放完成 |
| `CANCELLED` | 3 | 用户取消或超时取消 |

**所有状态流转都是「条件 UPDATE + 影响行数」**，例如：

```sql
UPDATE `order` SET status = 2, completed_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 1     -- 只有当前状态匹配才生效
```

因此重复回调、取消与支付并发这类场景天然幂等，不需要分布式锁。

支付单状态：`WAIT_PAY(0) / PAID(1) / CLOSED(2) / FAILED(3)`；
回调流水结果：`SUCCESS(1) / FAILED(2)`。

## 6. 数据库

库名 `order_db`（与 `product_db` 在同一个 MySQL 实例），建表脚本：`src/main/resources/db/schema.sql`（手工执行）。

| 表 | 说明 |
| --- | --- |
| `order` | 订单主表（`order` 是 MySQL 保留字，SQL 中一律用反引号） |
| `order_item` | 订单明细，保存商品/SKU 名称与单价快照 |
| `payment_order` | 支付单，`transaction_id` 唯一，防同一渠道流水被重复绑定 |
| `payment_record` | 支付回调流水，只追加不修改，用于对账 |

### 与《设计文档.md》的两处必要修正

1. `order_item.product_id` / `sku_id` 由 `BIGINT` 改为 `VARCHAR(32)`。
   **原因**：product-service 的 RPC 契约（`SkuDTO.productId` / `skuId`）对外只暴露业务编号，
   order-service 拿不到商品/SKU 的内部自增 ID；若强行用 BIGINT 会直接不兼容。
2. `payment_record` 的索引 `idx_payment_no (payment_no)` 修正为 `idx_payment_id (payment_id)`。
   **原因**：原 DDL 引用了不存在的列名。

## 7. 关键设计

| 主题 | 做法 |
| --- | --- |
| 价格篡改 | 单价/名称全部由服务端 RPC 查询后快照写入 `order_item`，请求体只接受 `skuId + quantity` |
| 下单预占 | `reserveStock` 逐 SKU 调用；**任一失败则补偿释放本次已成功的预占**（RPC 不在本地事务内，必须显式补偿） |
| 下单落库与 RPC 的顺序 | 先落库（拿订单主键）→ 再预占；预占失败抛异常回滚订单 |
| 取消/超时释放库存 | 释放动作注册到 **事务提交之后** 执行（`TransactionSynchronization#afterCommit`）：释放不可回滚，若在事务内执行而事务回滚，会因「订单仍是待支付」被二次释放导致可用库存超发 |
| 支付回调幂等 | 已支付直接确认；`markPaid`/`markCompleted` 均为条件更新；渠道流水号唯一 |
| 回调留痕 | `PaymentRecordWriter` 用 `REQUIRES_NEW` 独立事务写流水，外层因金额不符等异常回滚时留痕仍然保留 |
| 权益发放 | 支付成功后 `confirmStock`（锁定库存 → 已扣减）；成功才置 `COMPLETED`，失败则订单停在 `PAID` 由定时任务补偿重试 |
| 超时取消 | `OrderTimeoutTask` 每 60s 扫描 `status=0 AND expire_at <= now`（命中 `idx_status_expire_at`），逐单独立处理，单笔失败不影响整批 |
| 越权访问 | 他人订单返回 404 而非 403，避免探测 |
| 下游故障 | `ProductRpcClient` 把任意远端失败翻译成 503 / `RPC_INVOKE_FAILED`（Dubbo 的失败形态不止 `RpcException`，也包括懒连接不可达时的 `IllegalStateException`） |

## 8. 配置项

| 配置 | 默认值 | 说明 |
| --- | --- | --- |
| `server.port` | 8004 | |
| `spring.datasource.url` | `jdbc:mysql://localhost:3306/order_db` | |
| `order.expire-minutes` | 30 | 待支付订单超时时间 |
| `order.rpc.product-url` | `dubbo://127.0.0.1:20880` | product-service 直连地址 |
| `order.payment.callback-secret` | `order-service-callback-secret` | 回调验签密钥，生产必须通过环境变量注入 |
| `order.task.enabled` | true | 定时任务开关（测试关闭） |
| `order.task.scan-interval-ms` / `batch-size` | 60000 / 100 | 扫描间隔与单轮批量 |

**RPC 引用方式**：`@DubboReference(version = "1.0.0", check = false, lazy = true, url = "${order.rpc.product-url:...}")`。
- `check = false` + `lazy = true`：provider 未就绪时本服务照常启动，连接推迟到首次调用。
- 切到注册中心：删掉注解里的 `url` 属性，并把 `dubbo.registry.address` 配成 `nacos://...`。
  （注意：`url` 留空串**不行**，registry 为 `N/A` 时 Dubbo 会直接启动失败。）

## 9. 本地启动

```powershell
# 1. 建库建表
mysql -uroot -proot -e "CREATE DATABASE IF NOT EXISTS order_db DEFAULT CHARSET utf8mb4;"
mysql -uroot -proot order_db < src/main/resources/db/schema.sql

# 2. 启动依赖：nacos(8848)、redis(6379, 网关用)、product-service(8003, 暴露 20880)

# 3. 启动本服务（java.version=22，需 JDK 22）
$env:JAVA_HOME=""      # 若 JAVA_HOME 指向 JDK 17，需先清掉
mvn spring-boot:run
```

网关已加入路由 `lb://order-service`（`/api/v1/orders/**`、`/api/v1/payments/**`），
并把 `POST /api/v1/payments/callback` 放进了 JWT 白名单。经网关访问示例：

```powershell
curl.exe -X POST http://localhost:8000/api/v1/orders `
  -H "Authorization: Bearer <jwt>" -H "Content-Type: application/json" `
  -d '{\"items\":[{\"skuId\":\"S1ABC\",\"quantity\":2}]}'

curl.exe "http://localhost:8000/api/v1/orders?status=0&page=1&size=10" -H "Authorization: Bearer <jwt>"
```

## 10. 测试

```powershell
Remove-Item Env:JAVA_HOME -ErrorAction SilentlyContinue   # JDK 17 会因 release 22 编译失败
mvn clean test
```

12 个用例全部通过：

| 测试类 | 用例数 | 覆盖内容 |
| --- | --- | --- |
| `OrderHttpApiTest` | 4 | 全链路（创建→详情→我的订单→支付单→回调→已完成）、取消订单、支付单幂等、错误码与状态码映射（12 种） |
| `OrderDomainIntegrationTest` | 7 | 价格快照与预占、库存不足补偿、超时取消（含提交后释放与幂等）、回调幂等、权益发放失败留 `PAID` 后补偿完成、支付失败可重试、回调金额校验 |
| `OrderRpcWiringTest` | 1 | **不使用 mock**：校验 `@DubboReference` 真实注入 + 远端失败翻译为 `RPC_INVOKE_FAILED` |

两点刻意的测试设计：

- `OrderHttpApiTest` 用 `@MockBean ProductRpcClient` 隔离 product-service，跑得快；
- `OrderDomainIntegrationTest` **不加 `@Transactional`**：取消类操作的库存释放被安排到事务提交之后，
  若用事务包裹测试，释放永远不会触发、也就测不到真实行为；因此各用例使用互不相同的 UID / 订单编号隔离数据。

## 11. 已知限制

- **支付成功后订单被取消**（超时任务与回调并发）：订单会被置为 `CANCELLED`，回调侧只写 ERROR 日志并保留已支付的支付单，
  需要**人工/离线任务退款**（真实系统应引入退款单与对账）。
- **权益发放的补偿是「重试」而非「对账」**：若 `confirmStock` 成功但本地事务回滚，订单会长期停在 `PAID`
  （重试时 `confirmStock` 返回 false）。生产建议改为本地消息表 + 事务提交后投递。
- `payment_record` 只记录回调，不记录创建支付单等其它渠道交互（受设计文档表结构限制）。
- 定时任务为单实例扫描，多实例部署需加分布式锁或改用分布式调度。
