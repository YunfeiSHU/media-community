# order-service 订单中心

订单中心：**订单创建 / 查询 / 取消**、**支付单创建**、**支付回调**、**超时取消与权益发放补偿**。
商品与库存的数据所有权在 `product-service`，本服务通过 Dubbo 调 `ProductRpcService` / `StockRpcService`
完成可售性校验与库存预占 / 确认 / 释放，本地只持有订单相关的四张表。

## 1. 在整体架构中的位置

```
                     gateway (8000)
                        │  /api/v1/orders/**  /api/v1/payments/**
                        ↓
                 order-service (8004)
                        │
          ┌─────────────┼──────────────┐
          ↓             ↓              ↓
       Order        Payment      OrderTimeoutTask
          │             │         （60s 扫描）
          └──────┬──────┘
                 ↓
       MySQL auth_service（5 个 Java 服务共用库）

order-service ──Dubbo 直连 dubbo://127.0.0.1:20880──► product-service (8003)
                                                     Product / SKU / Inventory
```

- 服务一览、端口、中间件端口见 `../README.md` §2，本文不复述。
- 网关路由为 `lb://order-service`（本服务注册在 Nacos 8848），`POST /api/v1/payments/callback` 在 JWT 白名单内；
  网关转发前会清掉客户端伪造的 `X-User-*` 头。
- 只依赖 MySQL 与 Nacos，不依赖 Redis / RabbitMQ（Redis 仅网关限流使用）。

## 2. 技术栈

Spring Boot 3.3.5 / Java 17 / MyBatis（手写 XML，无 MyBatis-Plus）/ MySQL 8 / Dubbo 3.3.0 / Spring Cloud 2023.0.3 + Nacos。
测试用 H2（`MODE=MySQL`）+ MockMvc + `@MockBean`。
金额单位**分**，全程 `long`，用 `Math.multiplyExact` / `Math.addExact` 做溢出保护（越界 → `40001`）。

## 3. 目录结构

```
src/main/java/com/example/order
├── OrderServiceApplication.java    @SpringBootApplication @MapperScan @EnableScheduling
├── api/                            product-service 的 RPC 契约副本（ProductRpcService / StockRpcService + dto/）
├── common/ + common/http/          统一响应 / 错误码 / 业务异常 / ID 生成 / 当前用户 / 分页 / X-Request-Id 透传
├── config/                         Jackson 时间格式 / 配置项 / Dubbo
├── controller/                     OrderController  PaymentController
├── domain/entity|enums/            Order OrderItem PaymentOrder PaymentRecord
│                                   OrderStatus PaymentStatus PaymentChannel CallbackStatus SkuStatus ProductStatus
├── dto/request|response/           CreateOrderRequest CreatePaymentRequest PaymentCallbackRequest
│                                   OrderVO OrderItemVO PaymentVO
├── mapper/ + resources/mapper/*.xml    四张表的 Mapper 与 SQL
├── service/impl/                   OrderServiceImpl  PaymentServiceImpl
├── support/                        OrderLookup ProductRpcClient PaymentSigner PaymentRecordWriter
└── task/                           OrderTimeoutTask（超时取消 + 权益发放补偿）
```

## 4. HTTP 接口

统一响应 `{"code": 0, "message": "success", "data": ...}`（字段总则见 `../README.md` §5.1）；
接口清单与鉴权列见 `../README.md` §5.4「订单与支付」，本节只补报文与语义。

- 身份只取网关注入的 `X-User-Id`（值 = JWT 标准 claim `sub`），角色只取 `X-User-Role`（= `role` claim，入口 `CurrentUser.role()`）；
  服务不解析 JWT，**不接受**请求体 / 查询参数里的用户标识与角色；缺头 → `401 / 40101`。
- 直连 8004 可伪造这两个头，生产需在网络层禁止绕过网关；订单接口目前均未按角色鉴权。
- `{id}` 支持两种形态：订单业务编号（`O…`，`IdGenerator` 生成，长度 ≤ 24）或内部数字主键；
  他人的订单一律返回 `404 / 40401`，不泄露资源存在性（`OrderLookup.java:36-48`）。

### 4.1 创建订单

```http
POST /api/v1/orders
X-User-Id: u-1001
Content-Type: application/json

{
  "items": [
    {"skuId": "S1ABC...", "quantity": 2},
    {"skuId": "S2DEF...", "quantity": 1}
  ]
}
```

```json
{"code": 0, "message": "success", "data": {
  "id": 12, "orderId": "O2AB7K9Q3X1M", "uid": "u-1001", "status": 0, "statusName": "待支付",
  "totalAmount": 20800, "expireAt": "2026-09-22 18:08:05",
  "items": [{"id": 21, "productId": "P1", "skuId": "S1ABC...", "productName": "商品A",
             "skuName": "规格-默认", "unitPrice": 9900, "quantity": 2, "totalAmount": 19800}]}}
```

> **价格只信服务端**：`unitPrice` / `productName` / `skuName` 由 RPC 实时读取后写入 `order_item` 快照，
> 请求体即使传价格也被忽略；下单后商品改名改价不影响历史订单。
> 重复 SKU 拦截两次：先按请求原文（`normalizeItems`），拿到 RPC 返回的业务编号后再去一次重
> （`"12"` 与 `"S12abc"` 可能指向同一个 SKU，见 `OrderServiceImpl.java:92-104`）。

### 4.2 创建支付单

```http
POST /api/v1/orders/O2AB7K9Q3X1M/pay
X-User-Id: u-1001

{"paymentChannel": 1}      // 1-微信 2-支付宝 3-余额
```

金额取自订单 `total_amount`，支付单 `expire_at` 与订单一致；订单非待支付 → `40901`，已过 `expire_at` → `40905`。
**幂等**：已存在未支付的支付单则直接复用（`PaymentServiceImpl.java:69-79`），不重复创建。
渠道为模拟实现，本地演示不接真实渠道（`PaymentChannel` 类注释）。

### 4.3 支付回调报文

```http
POST /api/v1/payments/callback
Content-Type: application/json

{"paymentId": "P2AB7K9Q4Y2N", "transactionId": "WX-TX-0001", "paymentChannel": 1,
 "tradeStatus": "SUCCESS", "amount": 20800, "sign": "3f2a...（HmacSHA256 十六进制小写）"}
```

验签串（按字段名升序拼接，值为原始字面量）：

```
amount={amount}&paymentChannel={paymentChannel}&paymentId={paymentId}&tradeStatus={tradeStatus}&transactionId={transactionId}
```

本地构造合法回调：注入 `PaymentSigner` 调 `sign(new PaymentCallbackRequest(...))`，把结果填入 `sign`。
密钥由 `order.payment.callback-secret`（环境变量 `PAYMENT_CALLBACK_SECRET`）配置，**yml 里没有默认值**；
未配置时启动打 WARN、每次回调再打 WARN 并**跳过验签**，仅限本地联调。

### 4.4 查询与分页

- `GET /api/v1/orders/{id}`：按业务编号或内部主键查，不存在或非本人 → `404 / 40401`。
- `GET /api/v1/orders?status=&page=&size=`：`page` 默认 1（小于 1 归 1），`size` 默认 10、上限 50（≤0 按 10）；
  `status` 非法**先校验**直接 `40001`，不返回空列表（`OrderServiceImpl.java:161-180`）。
- 排序 `ORDER BY id DESC`；列表用 `selectByOrderIds` 一次批量取明细，避免 N+1（`OrderServiceImpl.java:364-375`）。

### 4.5 订单特有的错误码

前缀规范见 `../README.md` §5.3（`40001` 参数、`40101` 未认证、`40301` 无权限、`50000` 内部错误）。
以下是本服务独占的码，HTTP 状态码与 code 前三位对齐：

| code | HTTP | 含义 |
| --- | --- | --- |
| 40002 | 400 | 支付回调签名校验失败 |
| 40401 | 404 | 订单不存在（他人订单同样按不存在处理） |
| 40402 | 404 | 支付单不存在 |
| 40403 / 40404 | 404 | 商品不存在 / SKU 不存在（RPC 返回 null） |
| 40901 | 409 | 订单当前状态不允许该操作 |
| 40902 / 40903 | 409 | 商品未上架 / SKU 不可售或未设价格 |
| 40904 | 409 | 可用库存不足 |
| 40905 | 409 | 订单已超时 |
| 40906 | 409 | 回调金额与支付单金额不一致 |
| 40907 | 409 | 支付单状态不允许该操作 / 回调渠道与支付单不一致 |
| 40908 | 409 | 渠道流水号已被其它支付单占用 |
| 40909 | 409 | 唯一索引冲突（并发重复提交兜底，`DuplicateKeyException`） |
| 50001 | 503 | 远程服务（`product-service`）调用失败 |

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

**所有状态流转都是「条件 UPDATE + 影响行数」**（`OrderMapper.xml:37-65`）：

```sql
-- markCompleted：PAID(1) → COMPLETED(2)
UPDATE `order` SET status = 2, completed_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 1      -- 只有当前状态匹配才生效
```

因此重复回调、取消与支付并发这类场景天然幂等，不需要分布式锁；影响行数为 0 就是「已被别人处理」。
`markPaid` 同样带 `AND status = 0`，`markCancelled` 带 `AND status = 0`。

支付单状态：`WAIT_PAY(0) / PAID(1) / CLOSED(2) / FAILED(3)`（订单取消或超时时，未支付的支付单由
`closeWaitPayByOrderId` 一并关闭）；回调流水结果：`SUCCESS(1) / FAILED(2)`。

## 6. 支付回调：五段处理

入口 `PaymentServiceImpl#handleCallback`（`PaymentServiceImpl.java:99-161`），`@Transactional`。顺序固定：

| # | 段 | 实现位置 | 失败出口 |
| --- | --- | --- | --- |
| 1 | 验签 | `PaymentSigner#verify`（`PaymentSigner.java:74-86`） | `40002`，**不留痕**（来源不可信） |
| 2 | 幂等 | `PaymentServiceImpl.java:117-122` | 支付单已 `PAID`：留 `SUCCESS` 流水后直接返回 |
| 3 | 金额 | `PaymentServiceImpl.java:133-138` | 留 `FAILED` 流水 + `40906` |
| 4 | 流水号 | `PaymentServiceImpl.java:139-145` | 留 `FAILED` 流水 + `40908` |
| 5 | 条件更新 | `PaymentServiceImpl.java:147-154` + `OrderMapper.xml:38-45` | 影响行数 0 → 视为并发重复回调，直接确认 |

夹在 1 与 2 之间还有两次校验：支付单不存在 → `40402`；回调渠道与支付单不一致 → `40907`
（`PaymentServiceImpl.java:108-115`）。第 2 段之前的分支里，渠道明确回 `tradeStatus=FAILED` 会把支付单
置为 `FAILED` 并留痕（`PaymentServiceImpl.java:124-131`），用户可重新发起支付。

**验签：HmacSHA256 + 常量时间比较。** `Mac.getInstance("HmacSHA256")` 对上面的 `canonical` 串出小写十六进制
（`PaymentSigner.java:96-105`）；比较用 `MessageDigest.isEqual(...)` 而非 `String.equals`，避免逐字节短路
带来的时序侧信道（`PaymentSigner.java:80-85`）；渠道回传的 `sign` 先 `trim().toLowerCase()` 再比。
回调接口免登录，验签是这条链路**唯一**的鉴权手段，所以密钥故意不留默认值、只从环境变量注入。

**留痕用独立事务。** `PaymentRecordWriter#append` 标注 `REQUIRES_NEW`（`PaymentRecordWriter.java:35-46`）：
金额不符、流水号冲突等异常会让外层事务回滚，但「渠道确实调过来一次」必须落库供对账，因此流水单独提交。
`payment_record.callback_data` 存回调报文 JSON。

**支付成功但订单已被取消**：第 5 段成功、`orderService.markPaid` 的条件更新却影响 0 行时（超时任务抢先置了 `CANCELLED`），
`OrderServiceImpl.java:206-211` 只打 ERROR 日志，支付单保持 `PAID`、订单停在 `CANCELLED` 不再推进——
这笔钱需要人工 / 离线退款，见 §13。

## 7. 库存预占、确认与释放

| 阶段 | 触发 | 失败处理 |
| --- | --- | --- |
| 预占 `reserveStock` | 创建订单，逐 SKU 调用 | 任一失败 → 补偿释放本次已成功的预占，再抛 `40904`（`OrderServiceImpl.java:307-322`） |
| 确认 `confirmStock` | 支付回调 → `markPaid` → `completePaidOrder` | 返回 false → 订单停在 `PAID`，由补偿任务重试 |
| 释放 `releaseStock` | 用户取消 / 超时取消 | 事务**提交之后**执行；失败只打 ERROR 日志（需对账补偿） |

确认的顺序是**先 `confirmStock` 扣减、再改订单状态**（`OrderServiceImpl.java:232-251`）：中途失败订单停在 `PAID`，
由补偿任务重试；代价是「扣减已成功但改状态失败」不能自动收敛（见 §13）。

**下单顺序：先落库再预占。** 先 `INSERT order` 拿内部主键、批量写 `order_item`，再预占库存；预占失败抛异常回滚订单。
反过来（先预占后落库）一旦落库失败，锁定的库存没有订单可挂，只能靠对账回收。
预占发生在本地事务**内部**（远程写会一直占着数据库连接），是学习项目的取舍。

**releaseAfterCommit 的理由。** 取消 / 超时的释放动作注册到 `TransactionSynchronization#afterCommit`
（`OrderServiceImpl.java:324-341`）：释放库存是不可回滚的远程副作用，若在事务内执行而事务最终回滚，
订单仍是 `WAIT_PAY`，下一次取消会**再释放一次**，可用库存被超发。放到提交后，只有订单状态真正落库才释放。
`releaseQuietly` 让释放失败不阻断取消主流程，只记 ERROR（`orderDbId` 是 `order.id`，不是对外业务编号）。

## 8. 定时任务

`OrderTimeoutTask`，`@ConditionalOnProperty(order.task.enabled=true)`（缺省开启）。两个子任务共用
「批量扫描 → 逐单处理」：先按条件捞出候选订单编号，再逐个处理，单笔失败只记日志、下轮自然重试
（`OrderTimeoutTask.java:36-81`）。

| 任务 | 扫描条件 | 动作 |
| --- | --- | --- |
| 超时取消 `cancelExpiredOrders` | `status=0 AND expire_at<=now`（`idx_status_expire_at`），`ORDER BY id LIMIT batch-size` | 条件更新 `CANCELLED` → 关闭未支付支付单 → `afterCommit` 释放库存 |
| 权益发放补偿 `retryCompleteOrders` | `status=1 AND paid_at<=now` | `completePaidOrder`：`confirmStock` 全部成功才置 `COMPLETED` |

调度参数：`fixedDelay = order.task.scan-interval-ms`（60000）、`initialDelay = order.task.initial-delay-ms`（30000）、
单轮上限 `batch-size`（100）。测试环境用 `order.task.enabled=false` 关闭。

## 9. 数据库

库名 `auth_service`（与 `product-service` 等 5 个 Java 服务同一个 MySQL 实例、同一个库，靠表名区分）。
建表脚本 `src/main/resources/db/schema.sql`（`CREATE TABLE IF NOT EXISTS`，**手工执行**，不在 Spring 启动路径上）。
`order` 是 MySQL 保留字，所有 SQL 一律反引号。

```
   product-service                  order-service
   ├── product                      ├── order
   ├── sku                          ├── order_item
   └── inventory                    ├── payment_order
                                    └── payment_record

        └───── 同一个 MySQL 库 auth_service（靠表名区分归属）─────┘
```

```
order:谁在什么时候创建了哪个订单、多少钱、什么状态
  │ 1 : N
  ↓
order_item:这个订单具体买了什么（商品/SKU 名称与单价快照）

order
  │ 1 : N / 1 : 1
  ↓
payment_order:支付单
  │ 1 : N
  ↓
payment_record:支付渠道回调历史，只追加不修改
```

| 表 | 关键列 | 索引 / 唯一键 |
| --- | --- | --- |
| `order` | `id` `order_id` `uid` `status` `total_amount` `expire_at` `paid_at` `completed_at` `cancelled_at` `created_at` `updated_at` | `uk_order_id(order_id)`、`idx_uid_created_at`、`idx_status_expire_at` |
| `order_item` | `id` `order_id` `product_id` `sku_id` `product_name` `sku_name` `unit_price` `quantity` `total_amount` `created_at` | `idx_order_id`、`idx_product_id`、`idx_sku_id` |
| `payment_order` | `id` `payment_id` `order_id` `uid` `amount` `status` `payment_channel` `transaction_id` `expire_at` `paid_at` `created_at` `updated_at` | `uk_payment_id`、`uk_transaction_id`（防同一渠道流水被重复绑定）、`idx_order_id`、`idx_uid_created_at` |
| `payment_record` | `id` `payment_id` `transaction_id` `payment_channel` `callback_status` `callback_data` `created_at` | `idx_payment_id`、`idx_transaction_id` |

**相对初版 DDL 的两处修正**（`schema.sql:5-9`）：

1. `order_item.product_id` / `sku_id` 由 `BIGINT` 改为 `VARCHAR(32)`。原因：`SkuDTO.productId` / `skuId`
   对外只暴露业务编号（`P…` / `S…`），order-service 拿不到商品 / SKU 的内部自增 ID，`BIGINT` 直接不兼容。
2. `payment_record` 的 `idx_payment_no (payment_no)` 改为 `idx_payment_id (payment_id)`。原因：原 DDL 引用了不存在的列。

## 10. 配置项

| 配置 | 默认值 | 说明 |
| --- | --- | --- |
| `server.port` | 8004 | |
| `spring.datasource.url` | `jdbc:mysql://localhost:3306/auth_service` | 共用库 |
| `spring.cloud.nacos.discovery.server-addr` | `localhost:8848` | 网关 `lb://order-service` 依赖它 |
| `dubbo.registry.address` | `N/A` | 本地联调不注册；消费方端口 `-1` 随机，避免与 20880 冲突 |
| `order.expire-minutes` | 30 | 待支付订单超时时间，写入 `order.expire_at` |
| `order.rpc.product-url` | `dubbo://127.0.0.1:20880` | product-service 直连地址 |
| `order.payment.callback-secret` | 空 | `PAYMENT_CALLBACK_SECRET` 注入；**空则跳过验签** |
| `order.task.enabled` | true | 定时任务开关（测试关闭） |
| `order.task.scan-interval-ms` / `batch-size` / `initial-delay-ms` | 60000 / 100 / 30000 | 扫描间隔、单轮批量、首轮延迟 |
| `dubbo.consumer.timeout` / `retries` | 3000 / 0 | 超时 3s，**不重试**（避免重复预占/扣减） |

**RPC 引用方式**：`@DubboReference(version = "1.0.0", check = false, lazy = true, url = "${order.rpc.product-url:...}")`。

- `check = false` + `lazy = true`：provider 未就绪时本服务照常启动，连接推迟到首次调用。
- 切到 Nacos 注册中心：删掉 `order.rpc.product-url`（或删掉注解里的 `url` 属性），把 `DUBBO_REGISTRY_ADDRESS`
  配成 `nacos://localhost:8848`。Dubbo 中 `url` 优先级高于注册中心，**置空串不行**——空串会让 Dubbo 启动失败。

## 11. 本地启动

```powershell
# 1. 建库建表（库名 auth_service，与其他 Java 服务共用）
mysql -uroot -proot -e "CREATE DATABASE IF NOT EXISTS auth_service DEFAULT CHARSET utf8mb4;"
mysql -uroot -proot auth_service < src/main/resources/db/schema.sql

# 2. 启动依赖：nacos(8848)、product-service(8003, 暴露 20880)
#    Redis 6379 只有网关限流用，本服务不直连

# 3. 启动本服务（需 JDK 17）
Remove-Item Env:JAVA_HOME -ErrorAction SilentlyContinue   # JAVA_HOME 指向别的版本时先清掉
mvn spring-boot:run
```

网关路由 `/api/v1/orders/**`、`/api/v1/payments/**` → `lb://order-service`，回调路径在白名单。经网关访问：

```powershell
curl.exe -X POST http://localhost:8000/api/v1/orders `
  -H "Authorization: Bearer <jwt>" -H "Content-Type: application/json" `
  -d '{\"items\":[{\"skuId\":\"S1ABC\",\"quantity\":2}]}'

curl.exe "http://localhost:8000/api/v1/orders?status=0&page=1&size=10" -H "Authorization: Bearer <jwt>"
```

直连 8004 调试需手工补 `-H "X-User-Id: u-1001"`，否则 `401 / 40101`。

本地模拟一次支付回调（`sign` 由 `PaymentSigner#sign` 用同一个密钥算出，报文里的 `sign` 必须与金额、渠道、流水号一致）：

```powershell
curl.exe -X POST http://localhost:8004/api/v1/payments/callback `
  -H "Content-Type: application/json" `
  -d '{\"paymentId\":\"P2AB7K9Q4Y2N\",\"transactionId\":\"WX-TX-0001\",\"paymentChannel\":1,\"tradeStatus\":\"SUCCESS\",\"amount\":20800,\"sign\":\"<hex>\"}'
```

成功返回 `{"code":0,"message":"success"}`，同时 `payment_order` 置 `PAID`、`payment_record` 追加一条 `SUCCESS` 流水。

## 12. 测试

```powershell
Remove-Item Env:JAVA_HOME -ErrorAction SilentlyContinue   # 编译目标为 17，JDK 版本不符会直接失败
mvn clean test
```

16 个用例全部通过（`target/surefire-reports` 可复核）：

| 测试类 | 用例数 | 覆盖内容 |
| --- | --- | --- |
| `OrderHttpApiTest` | 4 | 全链路（创建→详情→我的订单→支付单→回调→已完成）、取消订单、支付单幂等、错误码与状态码映射 |
| `OrderDomainIntegrationTest` | 7 | 价格快照与预占、库存不足补偿、超时取消（提交后释放 + 幂等）、回调幂等、权益发放失败留 `PAID` 后补偿完成、支付失败可重试、回调金额校验 |
| `OrderRpcWiringTest` | 1 | **不使用 mock**：校验 `@DubboReference` 真实注入 + 远端失败翻译为 `50001` |
| `PaymentSignerTest` | 4 | 正确签名通过、错误/缺失签名 → `40002`、大小写与首尾空格归一化、未配密钥时跳过验签 |

两点刻意的测试设计：

- `OrderHttpApiTest` 用 `@MockBean ProductRpcClient` 隔离 product-service，跑得快；
- `OrderDomainIntegrationTest` **不加 `@Transactional`**：库存释放被安排到事务提交之后，用事务包住测试会让释放永不触发、
  也就测不到真实行为；因此各用例用互不相同的 UID / 订单编号隔离数据。

测试环境配置（`application-test.yml`）：H2 内存库 `jdbc:h2:mem:order_db;MODE=MySQL;NON_KEYWORDS=ORDER`
（`NON_KEYWORDS=ORDER` 让保留字也能当表名）、Nacos 自动注册关闭、`order.task.enabled=false`、
`order.payment.callback-secret=test-callback-secret`；`order.rpc.product-url` 特意指向 `dubbo://127.0.0.1:20999`
——避开本地常驻的 20880，否则 provider 在线时 `OrderRpcWiringTest` 会变成环境相关。

## 13. 已知缺陷与取舍

| 缺陷 | 现状与根因 | 位置 |
| --- | --- | --- |
| `confirmStock` 在事务内 | `completePaidOrder` 自身没有 `@Transactional`，但调用链 `handleCallback(@Transactional)` → `markPaid` → `completePaidOrder` 全都在这一个事务里，远程扣减会一直占着数据库连接；且「扣减已成功、本地事务回滚」时订单会长期停在 `PAID`，重试时 `confirmStock` 返回 false（已扣过）永远补不上 | `OrderServiceImpl.java:220-251`、`PaymentServiceImpl.java:33-36` |
| 创建支付单「先查后插」竞态 | `selectWaitPayByOrderId` 查不到才 `insert`，两个并发线程可能都查不到、都插入，同一订单出现两张 `WAIT_PAY` 支付单；`(order_id, status)` 不能做唯一键（`status` 会变） | `PaymentServiceImpl.java:69-79` |
| 支付成功但订单已被取消 | 条件更新影响 0 行（超时任务抢先置 `CANCELLED`），只打 ERROR 日志并保留已支付的支付单，需**人工 / 离线任务退款**；本仓库没有退款单与对账流程 | `OrderServiceImpl.java:206-211` |
| 创建订单没有幂等键 | 客户端超时重试会真造两张订单、预占两次库存；缺 `Idempotency-Key`（或客户端自带订单号）+ 唯一索引 | `OrderServiceImpl.java:76-82` |
| 补偿是「重试」不是「对账」 | `confirmStock` 成功但本地事务回滚时无补偿路径；且 `releaseQuietly` 释放失败只记日志 | `OrderServiceImpl.java:344-362` |
| 定时任务单实例 | 单进程 `@Scheduled` 扫描，多实例部署会重复处理（靠条件更新兜底但浪费 RPC），需分布式锁或分布式调度 | `OrderTimeoutTask.java:21-25` |
| 流水只记回调 | `payment_record` 不记录创建支付单等其它渠道交互 | `schema.sql:72-84` |
| 验签可被配置关掉 | `callback-secret` 为空时不报错、只打 WARN 并放行任意报文；回调接口本身免登录，此时链路上没有任何鉴权 | `PaymentSigner.java:50-62` |

一处可由唯一索引兜底但不彻底修复：并发重复回调、重复创建支付单触发的 `DuplicateKeyException` 统一映射为 `409 / 40909`
（`GlobalExceptionHandler.java:68-74`）。
