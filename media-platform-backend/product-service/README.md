# product-service 商品中心

Java 17 + Spring Boot 3.3 + MyBatis + Dubbo 3，覆盖**商品 / SKU / 价格 / 库存**四个域，
同时对外暴露 HTTP（`:8003`）与 Dubbo RPC（`:20880`，`version = "1.0.0"`）两套入口。
端口、快速开始、统一响应与错误码总表见 [../README.md](../README.md) §2 / §5.1 / §5.3，本文只写本服务的实现细节。

## 1. 目录结构

```text
src/main/java/com/example/product
├── api/            Dubbo 接口 + DTO + @DubboService 实现
├── common/         统一响应、错误码、BizException、业务编号生成；http/ 下 RequestIdFilter 写 MDC
├── controller/     ProductController、SkuController（17 个 HTTP 接口）
├── domain/         entity（product / sku / inventory）+ enums
├── dto/            request（record + Jakarta Validation）/ response
├── mapper/         MyBatis 接口，SQL 在 resources/mapper/*.xml
├── service/ + impl Product / Sku / Price / Stock
└── support/        EntityLookup（ID 或编号解析）、ProductOwnership（归属校验）

src/main/resources: application.yml · db/schema.sql · logback-spring.xml · mapper/*.xml
```

## 2. 数据模型

三个域是一条链：商品决定「谁在卖什么」，SKU 决定「卖哪个规格、多少钱」，库存决定「还能卖几件」。

```text
product（商品是什么、属于谁） 1 ──── N sku（卖什么规格、卖多少钱） 1 ──── 1 inventory（多少可卖、多少被订单预占）
```

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `product` | `id`(内部主键) `product_id`(业务编号 `P…`) `uid` `product_type` `status` | `product_type`：1-实物 2-虚拟；`status`：0-下架 1-上架 2-已删除 |
| `sku` | `id` `sku_id`(业务编号 `S…`) `product_id`(=`product.id`) `price`(分) `status` | `status`：0-禁用 1-启用 2-已删除；`uk_sku_id` 唯一 |
| `inventory` | `id` `sku_id`(=`sku.id`) `available_stock` `locked_stock` | 一个 SKU 一条记录，`uk_sku_id` 唯一；只有可用 / 锁定两态 |

> 设计稿里两处唯一索引写成了表中不存在的列（`uk_product_no (product_no)`、`uk_sku_no (sku_no)`），`db/schema.sql` 已改为
> `uk_product_id (product_id)` / `uk_sku_id (sku_id)`：以脚本为准。索引：`idx_uid_status(uid,status)`、`idx_type_status(product_type,status)`、`idx_product_id`、`idx_product_status(product_id,status)`。

**为什么两套编号**：内部主键 `BIGINT` 给外键与 join 用，业务编号 `VARCHAR(32)` 对外（`IdGenerator` 生成 `P…` / `S…`：
36 进制时间戳 + 12bit 自增 + 6 位随机，长度 ≤24），避免暴露自增 ID 与数据体量；代价是每个入口都要判一次「主键还是编号」，
判定集中在 `support/EntityLookup`。

## 3. HTTP 接口（17 个）

统一响应 `{"code":0,"message":"success","data":{...}}`。`spring.jackson.default-property-inclusion: non_null`
意味着 `data` 为 `null` 时字段整体消失——删除接口返回的是 `{"code":0,"message":"success"}`，断言时不要写 `"data": null`。

| 方法 | 路径 | 一句话职责 |
| --- | --- | --- |
| POST | `/api/v1/products` | 创建商品；创建者取网关注入的 `X-User-Id`，初始状态下架(0) |
| GET | `/api/v1/products/{id}` | 商品详情，含 SKU 列表与各自库存（一次批量查库存，避免 N+1） |
| PUT | `/api/v1/products/{id}` | 改名称/描述/封面/类型，字段可选，未传保持原值 |
| DELETE | `/api/v1/products/{id}` | 逻辑删除商品（`status=2`），级联逻辑删除其 SKU |
| POST | `/api/v1/products/{id}/on-shelf` | 上架，要求至少一个启用中的 SKU |
| POST | `/api/v1/products/{id}/off-shelf` | 下架 |
| POST | `/api/v1/products/{id}/skus` | 创建 SKU，事务内同步建 1:1 库存（`available=initialStock`、`locked=0`） |
| GET | `/api/v1/skus/{id}` | SKU 详情，含库存 |
| PUT | `/api/v1/skus/{id}` | 改 SKU 名称，或在禁用/启用之间切换（删除只能走 DELETE） |
| DELETE | `/api/v1/skus/{id}` | 逻辑删除 SKU；商品上架中或存在锁定库存则拒绝 |
| GET | `/api/v1/skus/{id}/price` | 查当前价格，即 `sku.price`，单位分，币种固定 CNY |
| PUT | `/api/v1/skus/{id}/price` | 改价，价格未变化时幂等返回 |
| GET | `/api/v1/skus/{id}/stock` | 查可用库存与锁定库存 |
| POST | `/api/v1/skus/{id}/stock/increase` | 补货：`available_stock + n` |
| POST | `/api/v1/skus/{id}/stock/reserve` | 预占：可用 → 锁定（HTTP 镜像接口，交易链路走 Dubbo） |
| POST | `/api/v1/skus/{id}/stock/release` | 释放：锁定 → 可用（订单取消 / 超时） |
| POST | `/api/v1/skus/{id}/stock/confirm` | 确认扣减：消耗锁定部分（支付成功） |

`{id}` 同时支持**内部主键**与**业务编号**：纯数字（`^\d{1,19}$`）走 `selectById`，其余按 `product_id` / `sku_id` 查，
由 `Identifiers` + `EntityLookup` 统一解析，前端拿到哪种都能用；查不到或已删除统一返回 `40401`（商品）/ `40402`（SKU）。

**仅创建者**：12 个写接口（改删商品、上下架、建 SKU、改删 SKU、改价、加/预占/释放/确认库存）要求
`product.uid == X-User-Id`，由 `ProductOwnership.requireProductOwner` / `requireSkuOwner` 校验，
SKU / 价格 / 库存的归属按其所属商品判断；不匹配返回 `403 / 40301`（`无权操作他人商品: P…`）。
4 个只读接口（商品详情、SKU 详情、价格、库存）与「创建商品」不做归属校验；**角色不等于归属**：即使是 `ADMIN`，也要是该商品的创建者才能写。

请求体：`CreateProductRequest`（`name` ≤128、`description` ≤500、`coverUrl` ≤512、`productType` 必填，**没有 `uid`**）、
`CreateSkuRequest`（`name` ≤128、`price` ∈ [0, 99999999999]、`initialStock` 缺省 0）、`UpdatePriceRequest`（`price`）、`StockChangeRequest`（`quantity` ≥1）；`@Valid` 失败统一 `40001`。

错误码：`40001` 参数不合法、`40101` 未登录（缺 `X-User-Id`）、`40301` 无权限、`40401/40402/40403` 商品/SKU/库存不存在、
`40901/40902` 商品/SKU 状态冲突、`40903` 可用库存不足、`40904` 锁定库存不足、`40909` 业务编号唯一键冲突、`50000` 系统异常；
前三位与 HTTP 状态码一致，完整表见 [../README.md](../README.md) §5.3。

```bash
# 创建商品（请求体里没有 uid，创建者来自网关注入头）
curl -X POST http://localhost:8003/api/v1/products -H "X-User-Id: u-1001" -H "Content-Type: application/json" \
  -d '{"name":"iPhone 17","description":"新品","productType":1}'
# 其余写接口同理：直连时手补 X-User-Id，经网关则用 Authorization: Bearer <jwt>
```

## 4. 库存两态与三条条件更新

`inventory` 只有两个数字：`available_stock`（可卖）与 `locked_stock`（被订单预占）。所有变更都只在这两态之间搬运：

```text
        reserve：available -n / locked +n          confirm：locked -n
available_stock ──────────────────────▶ locked_stock ──────────────▶ 已售出（库存净减少）
        ▲                                     │
        └─────────────────────────────────────┘
                 release：locked -n / available +n
```

三条 SQL 的**条件写在 `WHERE` 里**，没有「先 SELECT 再算」：

```sql
-- reserve：预占，可用够才扣
UPDATE inventory
SET available_stock = available_stock - #{quantity},
    locked_stock    = locked_stock + #{quantity}
WHERE sku_id = #{skuId} AND available_stock >= #{quantity};

-- confirm：确认扣减，只消耗锁定部分
UPDATE inventory
SET locked_stock = locked_stock - #{quantity}
WHERE sku_id = #{skuId} AND locked_stock >= #{quantity};

-- release：释放，锁定够才还回可用
UPDATE inventory
SET locked_stock    = locked_stock - #{quantity},
    available_stock = available_stock + #{quantity}
WHERE sku_id = #{skuId} AND locked_stock >= #{quantity};
```

**影响行数就是成败判定**：`InventoryMapper` 返回 `0` 行说明条件不成立（可用不足 / 锁定不足），
`StockServiceImpl` 直接抛 `BizException`（`40903` / `40904`），事务回滚；影响行数非 0 才算成功，并重新查一次库存回填响应。
`quantity <= 0` 在进 SQL 之前就返回 `40001`。库存类接口统一返回 `StockVO`：`skuId`（业务编号）、`availableStock`、`lockedStock`、`updatedAt`。

为什么这样写：
- 行锁 + 条件更新让「检查」和「扣减」在同一条语句里完成，并发下不可能超卖；拆成 `SELECT` 再 `UPDATE` 立刻就有窗口。
- 不用 Redis / Lua，也不用分布式锁：库存是强一致数据，落在 MySQL 里少一个要同步的副本；三条 update 都只碰一行。
- 条件不成立返回 0 行而不是抛 SQL 异常，失败语义留给业务层定：HTTP 返 `409`，Dubbo 返 `false`。

## 5. Dubbo 接口

```java
public interface ProductRpcService {
    ProductDTO getProduct(String productId);   // 不存在/已删除 → null
    SkuDTO getSku(String skuId);               // 不存在/已删除 → null
    List<SkuDTO> getSkus(String productId);    // 商品不存在 → 空集合
}

public interface StockRpcService {
    StockDTO getStock(String skuId);                   // 不存在 → null
    boolean reserveStock(String skuId, int quantity);  // 预占：可用 → 锁定
    boolean confirmStock(String skuId, int quantity);  // 支付成功：消耗锁定
    boolean releaseStock(String skuId, int quantity);  // 取消/超时：锁定 → 可用
}
```

- 约定：查询类返回 `null` / 空集合而**不抛异常**；库存变更类把 `BizException` 收敛成 `false`（`StockRpcServiceImpl.execute`），
  语义是「SKU 不存在 / 库存记录缺失 / 库存不足，业务未发生变更」，便于上游判断是否补偿，也不会因一次库存不足触发 RPC 重试。
- `skuId` / `productId` 参数同样接受内部主键或业务编号（走 `EntityLookup`）；消费端用 `@DubboReference(version = "1.0.0")` 注入即可。
- 实现类 `api/impl/ProductRpcServiceImpl`、`StockRpcServiceImpl`，均标 `@DubboService(version = "1.0.0")`；
  配置 `dubbo.registry.address=N/A`（单机免注册中心，直连暴露 20880）、`retries=0`、`timeout=3000`，
  接注册中心时改成 `nacos://127.0.0.1:8848`。
- RPC 是**交易链路入口**（order-service 查商品、锁库存），没有用户上下文，**不做归属校验**，也不经过 Servlet 过滤器。

## 6. 状态机与业务规则

**商品**
- 新建即「下架(0)」，配好 SKU 再上架；上架前校验至少一个启用中的 SKU，否则 `40901`。
- 上架状态不允许删除；删除是逻辑删除（`status=2`），并级联把其 SKU 置为已删除。
- 状态流转一律 `UPDATE ... WHERE id=? AND status=<读到的原状态>`，影响行数为 0 说明被并发改过，返回 `40901`「请刷新后重试」。

**SKU**
- 与库存 1:1，建 SKU 与建库存记录在同一事务里。
- 商品上架中不能删 SKU；有订单锁定库存（`locked_stock > 0`）也不能删，两者都是 `40902`。
- 更新接口不接受 `status=2`，删除必须走 DELETE，避免绕开上面两条校验。

**价格**
- 单位**分**（`BIGINT`），当前价格就是 `sku.price`；没有独立价格历史表，改价即覆盖，订单侧靠下单时快照。

## 7. 网关接入与鉴权

网关路由 `product-service` ← `Path=/api/v1/products/**,/api/v1/skus/**`，按服务名 `lb://product-service` 转发（本服务注册到 Nacos）。

| 接口 | 是否免鉴权 |
| --- | --- |
| `GET /api/v1/products/{id}`、`GET /api/v1/skus/{id}` | 无需 JWT（网关白名单） |
| 其余 15 个（写操作 + 价格 / 库存查询） | 必须携带 `Authorization: Bearer <jwt>` |

```yaml
gateway:
  auth:
    permit-all:
      - GET /api/v1/products/*   # 仅 GET 放行；PUT/DELETE 同路径仍需 JWT
      - GET /api/v1/skus/*       # Ant 的 * 只匹配单段，/skus/{id}/price、/stock 仍需 JWT
```

> 白名单分支会**先清掉**客户端可能伪造的 `X-User-Id` / `X-User-Role`（以及历史头 `uid`）再放行，因此白名单接口里读不到用户身份。
> 网关只注入这两个头（uid 取 JWT 标准 claim `sub`），服务自身不解析 JWT：**直连 8003 会绕过鉴权**，生产环境必须在网络层禁止该端口对外；
> 限流与白名单的完整规则见 [`gateway-service/技术文档.md`](../gateway-service/技术文档.md)。

## 8. 运行与测试

```bash
mysql -uroot -p < src/main/resources/db/schema.sql   # 1. 初始化（MySQL 8）
# 2. 按需修改 application.yml 的数据源账号密码
mvn spring-boot:run                                  # 3. 启动：HTTP 8003 + Dubbo 20880
mvn test                                             # 4. 测试：H2 内存库，不依赖 MySQL / Nacos
```

> 单机验证且不启动 Nacos 时设 `spring.cloud.nacos.discovery.register-enabled=false`；`application-test.yml` 已默认关闭注册与发现，并用 H2（`MODE=MySQL`）+ `db/schema-h2.sql`。

共 19 个测试用例，`mvn test` 全绿：`ProductDomainIntegrationTest`（领域服务 + RPC，H2 + 方法级事务回滚，覆盖 CRUD/上下架/级联删除、
库存四种变更与不足场景、RPC 契约）、`ProductHttpApiTest`（MockMvc 契约：17 个路由、响应结构、错误码映射，以及非创建者调 12 个写接口一律 `403 / 40301`）、`CurrentUserTest`（缺身份 `40101`、角色不匹配 `40301`）。

## 9. 日志与链路追踪

约定与 `gateway-service` / `user-service` / `auth-service` 一致：日志落 `logs/product-service/product-service.log`
（按天滚动、保留 30 天，目录可用环境变量 `LOG_PATH` 覆盖），同时输出控制台；每行带 `requestId=%X{requestId}`：

```text
2026-09-22 16:00:45.441 [http-nio-8003-exec-1] INFO  c.e.p.service.impl.ProductServiceImpl requestId=659af9be-... - 创建商品成功: productId=P2QP..., uid=u-1001, ...
```

- `RequestIdFilter`（`common/http`）在请求最外层把 `X-Request-Id` 写入 MDC（key `requestId`）并回写响应头，未携带时生成 UUID；
  网关已完成生成与透传，因此经网关进来的请求可用同一个 `requestId` 串起网关访问日志与本服务日志。Dubbo 调用不经过该过滤器，MDC 不跨 RPC 传播，RPC 侧日志用 `[RPC]` 前缀标识。
- **没有跨进程链路追踪**：未接入 OpenTelemetry / Jaeger，也没有 Prometheus 应用指标（`deploy/k8s` 里的
  `/stats/prometheus` 只是 Envoy sidecar 的注解，与业务无关）。分布式追踪属规划项、尚未实现。
- 级别：`com.example.product=info`（商品/SKU/价格/库存变更、RPC 调用）、`com.example.product.mapper=debug`（打印 SQL 与参数）、
  `root=INFO`；异常统一由 `GlobalExceptionHandler` 输出：业务异常 `warn`（含错误码），系统异常 `error`（含堆栈）。

## 10. 已知限制

| 项 | 说明 |
| --- | --- |
| 单库单表 | 无分库分表、无在途/预分配库存；库存行是热点行，高并发下靠行锁排队 |
| 归属校验只在 HTTP 层 | Dubbo 入口信任调用方，越权防护依赖内网隔离 |
| 建表需手工导入 | `db/schema.sql` 不在 Spring 默认执行路径上，见 [`deploy/db/README.md`](../deploy/db/README.md) |
