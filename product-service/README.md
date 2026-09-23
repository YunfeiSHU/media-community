# product-service 商品中心

Java 17 + Spring Boot 3.3 + MyBatis + Dubbo 3 的商品服务，覆盖**商品 / SKU / 价格 / 库存**四个域，
同时对外暴露 **HTTP（Restful）** 与 **Dubbo RPC** 两套入口。

## 1. 目录结构

```
src/main/java/com/example/product
├── ProductServiceApplication.java   启动类（@MapperScan）
├── api/                             Dubbo 对外接口 + DTO + 实现
│   ├── ProductRpcService.java       getProduct / getSku / getSkus
│   ├── StockRpcService.java         getStock / reserveStock / confirmStock / releaseStock
│   ├── dto/                         ProductDTO / SkuDTO / StockDTO（Serializable POJO）
│   └── impl/                        @DubboService 实现
├── common/                          统一响应、错误码、业务异常、全局异常处理、ID生成
│   └── http/                        链路 ID：RequestIdFilter + RequestIdConstants（写入 MDC）
├── config/                          Dubbo / Jackson 配置
├── controller/                      ProductController、SkuController
├── domain/                          entity（product / sku / inventory）+ enums
├── dto/request|response             出入参对象（record + Jakarta Validation）
├── mapper/                          MyBatis Mapper 接口（SQL 在 resources/mapper/*.xml）
├── service/ + service/impl/         ProductService / SkuService / PriceService / StockService
└── support/EntityLookup.java        「内部ID 或 业务编号」统一解析 + 存在性校验

src/main/resources
├── application.yml
├── logback-spring.xml               控制台 + 滚动文件，日志行带 requestId
├── db/schema.sql                    MySQL 建表脚本（与设计方案一致，修正了唯一索引列名笔误）
└── mapper/*.xml                     3 张表的 SQL 映射
```

## 2. 数据模型

```
product（商品是什么、属于谁） 1 ──── N sku（卖什么规格、卖多少钱） 1 ──── 1 inventory（多少可卖、多少被订单预占）
```

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `product` | `product_id`(业务编号) `uid` `product_type` `status` | `status`：0-下架 1-上架 2-已删除 |
| `sku` | `sku_id`(业务编号) `product_id`(商品内部ID) `price`(分) `status` | `status`：0-禁用 1-启用 2-已删除 |
| `inventory` | `sku_id`(SKU内部ID) `available_stock` `locked_stock` | 一个 SKU 一条记录，可用+锁定 |

> 建表脚本修正了设计方案中的两处笔误：`uk_product_no (product_no)` → `uk_product_no (product_id)`、
> `uk_sku_no (sku_no)` → `uk_sku_no (sku_id)`（原列名不存在）。

## 3. HTTP 接口

统一响应结构：`{"code":0,"message":"success","data":{...}}`；失败时 `code` 为业务错误码，
HTTP 状态码与错误码前三位一致（400/404/409/500）。

| 功能 | 方法 | 接口 | 要求 |
| --- | --- | --- | --- |
| 创建商品 | POST | `/api/v1/products` | 登录（`X-User-Id`） |
| 商品详情 | GET | `/api/v1/products/{id}` | 公开 |
| 修改商品 | PUT | `/api/v1/products/{id}` | 仅创建者 |
| 删除商品 | DELETE | `/api/v1/products/{id}` | 仅创建者 |
| 上架 | POST | `/api/v1/products/{id}/on-shelf` | 仅创建者 |
| 下架 | POST | `/api/v1/products/{id}/off-shelf` | 仅创建者 |
| 创建 SKU | POST | `/api/v1/products/{id}/skus` | 仅创建者 |
| SKU详情 | GET | `/api/v1/skus/{id}` | 公开 |
| 修改 SKU | PUT | `/api/v1/skus/{id}` | 仅创建者 |
| 删除 SKU | DELETE | `/api/v1/skus/{id}` | 仅创建者 |
| 当前价格查询 | GET | `/api/v1/skus/{id}/price` | 公开 |
| 修改价格 | PUT | `/api/v1/skus/{id}/price` | 仅创建者 |
| 查询库存 | GET | `/api/v1/skus/{id}/stock` | 公开 |
| 增加库存 | POST | `/api/v1/skus/{id}/stock/increase` | 仅创建者 |
| 库存扣减（预占） | POST | `/api/v1/skus/{id}/stock/reserve` | 仅创建者 |
| 库存释放 | POST | `/api/v1/skus/{id}/stock/release` | 仅创建者 |
| 确认扣减 | POST | `/api/v1/skus/{id}/stock/confirm` | 仅创建者 |

“仅创建者”指 `product.uid` 必须等于网关注入的 `X-User-Id`，由
[`ProductOwnership`](src/main/java/com/example/product/support/ProductOwnership.java) 统一校验；
不匹配时返回 `403 / 40301`（`无权操作他人商品: P…`）。SKU/价格/库存的归属按其所属商品判断。
只读查询（商品详情、SKU 详情、价格、库存）仍对所有人开放；
Dubbo 侧的 `ProductRpcService` / `StockRpcService` 是交易链路入口，没有用户上下文，**不做归属校验**。

`{id}` 同时支持**内部主键**（纯数字，如 `1`）与**业务编号**（如 `P2QP...`、`S2QP...`），
由 `Identifiers` + `EntityLookup` 统一解析，方便前端拿到哪种就用哪种。

错误码：`40001` 参数不合法，`40101` 未登录（缺 `X-User-Id` 头），`40301` 没有操作权限（不是商品创建者 / 角色不足），
`40401/40402/40403` 商品/SKU/库存不存在，
`40901/40902` 商品/SKU 状态冲突，`40903` 可用库存不足，`40904` 锁定库存不足，`50000` 系统异常。

> 当前用户（uid）**只从网关注入的 `X-User-Id` 头取**（值 = JWT 的 `sub`，见
> `com.example.product.common.CurrentUser`），不解析 JWT、也不接受请求体里的 `uid`。
> 直连 8081 调试写接口时需自带 `-H "X-User-Id: u-1001"`；
> 用**非创建者**的 uid 调写接口会得到 `403 / 40301`。
>
> 角色（`ADMIN` / `user`）同理：**唯一来源是网关注入的 `X-User-Role`**（值 = JWT 的 `role` claim），
> 通过 `CurrentUser.role()` 读取；需要管理员权限时用 `CurrentUser.requireAdmin()`（角色不足返回 `403 / 40301`）。
> 角色不等于归属：即使 `ADMIN`，写接口仍要求是该商品的创建者。

### 调用示例

```bash
# 创建商品（创建者 = 网关注入的 X-User-Id，请求体里没有 uid）
curl -X POST http://localhost:8081/api/v1/products -H "X-User-Id: u-1001" -H "Content-Type: application/json" -d '{
  "name": "iPhone 17", "description": "新品",
  "coverUrl": "https://cdn.example.com/1.png", "productType": 1 }'

# 创建 SKU（初始库存 10，需为商品创建者）
curl -X POST http://localhost:8081/api/v1/products/{productId}/skus -H "X-User-Id: u-1001" -H "Content-Type: application/json" \
  -d '{"name": "256G 黑色", "price": 699900, "initialStock": 10}'

# 上架 / 下架（需为商品创建者）
curl -X POST http://localhost:8081/api/v1/products/{productId}/on-shelf -H "X-User-Id: u-1001"
curl -X POST http://localhost:8081/api/v1/products/{productId}/off-shelf -H "X-User-Id: u-1001"

# 价格
curl -X PUT http://localhost:8081/api/v1/skus/{skuId}/price -H "X-User-Id: u-1001" -H "Content-Type: application/json" -d '{"price": 699900}'

# 库存：扣减 / 释放 / 增加（HTTP 镜像接口，需为商品创建者）
curl -X POST http://localhost:8081/api/v1/skus/{skuId}/stock/reserve -H "X-User-Id: u-1001" -H "Content-Type: application/json" -d '{"quantity": 2}'
curl -X POST http://localhost:8081/api/v1/skus/{skuId}/stock/release -H "X-User-Id: u-1001" -H "Content-Type: application/json" -d '{"quantity": 2}'
curl -X POST http://localhost:8081/api/v1/skus/{skuId}/stock/increase -H "X-User-Id: u-1001" -H "Content-Type: application/json" -d '{"quantity": 100}'
```

## 4. Dubbo 接口

```java
public interface ProductRpcService {
    ProductDTO getProduct(String productId);   // 不存在/已删除 → null
    SkuDTO getSku(String skuId);               // 不存在/已删除 → null
    List<SkuDTO> getSkus(String productId);    // 商品不存在 → 空集合
}

public interface StockRpcService {
    StockDTO getStock(String skuId);                        // 不存在 → null
    boolean reserveStock(String skuId, int quantity);       // 预占：可用 → 锁定
    boolean confirmStock(String skuId, int quantity);       // 支付成功：消耗锁定
    boolean releaseStock(String skuId, int quantity);       // 取消/超时：锁定 → 可用
}
```

RPC 约定：查询类返回 `null`/空集合而不抛异常；库存变更类返回 `false` 表示
「SKU 不存在 / 库存记录缺失 / 库存不足」，业务未发生变更，便于上游做补偿判断。

- 实现类：`api/impl/ProductRpcServiceImpl`、`api/impl/StockRpcServiceImpl`，均标注 `@DubboService(version = "1.0.0")`。
- 配置：`application.yml` 中 `dubbo.registry.address=N/A`（单机免注册中心，直接暴露；端口 20880）。
  接入注册中心时改为 `nacos://127.0.0.1:8848` 即可。

消费端示例：

```java
@DubboReference(version = "1.0.0")
private ProductRpcService productRpcService;

@DubboReference(version = "1.0.0")
private StockRpcService stockRpcService;
```

## 5. 业务规则与并发控制

**商品**
- 创建后初始状态为「下架」（`0`），配置完 SKU 再上架。
- 上架前校验商品下**至少有一个启用中的 SKU**，否则返回 `40901`。
- 上架状态下不允许删除商品；删除为逻辑删除（`status=2`），并级联逻辑删除其 SKU。
- 状态流转使用 `UPDATE ... WHERE id=? AND status=<原状态>` 做乐观校验，影响行数为 0 说明被并发修改，返回 `40901`。

**SKU**
- 创建 SKU 时事务内同步创建 1:1 的库存记录（`available=initialStock`, `locked=0`）。
- 商品上架时要先下架才能删除 SKU；存在订单锁定库存的 SKU 不允许删除。

**价格**
- 价格单位为**分**（`BIGINT`），当前价格即 SKU 售价；价格未变化时幂等返回。

**库存（防超卖）**
- 不使用「先查后改」，而是 `UPDATE inventory SET available_stock = available_stock - #{qty}
  WHERE sku_id = ? AND available_stock >= #{qty}`，由数据库行锁保证原子性，影响行数为 0 即库存不足。
- 语义：`reserve` 可用→锁定，`confirm` 消耗锁定，`release` 锁定→可用，`increase` 补货。
  任一环节失败都会抛出 `BizException`，事务整体回滚，不会出现中间态。

## 6. 运行

```bash
# 1. 初始化数据库（MySQL 8）
mysql -uroot -p < src/main/resources/db/schema.sql
# 2. 按需修改 application.yml 中的数据源账号密码
# 3. 启动（HTTP 8081，Dubbo 20880）
mvn spring-boot:run
```

> HTTP 接口同时会以服务名 `product-service` 注册到 Nacos，供网关用 `lb://product-service` 路由。
> 若只想单机验证 HTTP 接口、不启动 Nacos，可临时设 `spring.cloud.nacos.discovery.register-enabled=false`；
> 测试 profile（`application-test.yml`）已默认关闭注册与发现。

## 7. 测试

```bash
mvn test
```

- `ProductDomainIntegrationTest`：领域服务 + RPC 服务的集成测试（H2 内存库 + 方法级事务回滚），
  覆盖商品 CRUD/上下架/级联删除、SKU CRUD、价格读写、库存四种变更与不足场景、RPC 契约。
- `ProductHttpApiTest`：MockMvc 契约测试，逐个校验设计方案中约定的路由、响应结构与错误码映射；
  并单独覆盖**归属校验**（非创建者调 12 个写接口一律 `403 / 40301`，只读接口仍开放，创建者本人正常操作）。
- `CurrentUserTest`：身份/角色只从网关注入头解析的单元测试（缺身份 → `40101`，角色不匹配 → `40301`）。

当前共 19 个测试用例，`mvn test` 全绿。

## 8. 日志与链路追踪

与 `gateway-service` / `user-service` / `auth-service` 保持同一套约定。

**输出目标**（`logback-spring.xml`）

| 目标 | 说明 |
| --- | --- |
| 控制台 | 开发时直接观察 |
| `logs/product-service/product-service.log` | 按天滚动，保留 30 天；目录可用环境变量 `LOG_PATH` 覆盖 |

日志格式：

```text
2026-09-22 16:00:45.441 [http-nio-8081-exec-1] INFO  c.e.p.service.impl.ProductServiceImpl requestId=659af9be-... - 创建商品成功: productId=P2QP..., uid=u-1001, ...
```

**链路 ID**

- `RequestIdFilter`（`com.example.product.common.http`）在请求最外层把 `X-Request-Id` 写入 MDC（key = `requestId`），
  并回写同名响应头；请求未携带时自动生成 UUID。
- 网关已完成 `X-Request-Id` 的生成与透传，因此**经网关进来的请求天然贯通**：
  网关访问日志与各下游服务日志可用同一个 `requestId` 串联。
- 注意：Dubbo RPC 调用不经过该 Servlet 过滤器，MDC 不会跨 RPC 传播；RPC 侧日志用 `[RPC]` 前缀标识。

**日志级别**（`application.yml`）

| 包 / 级别 | 说明 |
| --- | --- |
| `com.example.product` = `info` | 业务关键动作：商品/SKU 增删改、上下架、价格变更、库存四种变更、RPC 调用 |
| `com.example.product.mapper` = `debug` | MyBatis 执行的 SQL 与参数（需要排查 SQL 时打开） |
| `root` = `INFO` | 兜底 |

异常日志统一由 `GlobalExceptionHandler` 输出：业务异常 `warn`（含错误码），系统异常 `error`（含堆栈）。

## 9. 网关接入与鉴权

HTTP 接口统一经 `gateway-service`（`:8000`）暴露，网关侧配置见其 `技术文档.md`。

| 路由 | 断言 |
| --- | --- |
| `product-service` | `Path=/api/v1/products/**,/api/v1/skus/**` |

**鉴权规则：详情公开，写操作必须过 JWT**

| 接口 | 是否免鉴权 |
| --- | --- |
| `GET /api/v1/products/{id}`（商品详情） | ✅ 无需 JWT |
| `GET /api/v1/skus/{id}`（SKU 详情） | ✅ 无需 JWT |
| 其余全部（创建/修改/删除商品、上下架、创建/改删 SKU、价格、库存） | ❌ 必须携带 `Authorization: Bearer <jwt>` |

免鉴权通过网关白名单实现，写法限定 HTTP 方法：

```yaml
gateway:
  auth:
    permit-all:
      - GET /api/v1/products/*   # 仅 GET 放行；PUT/DELETE 同路径仍需 JWT
      - GET /api/v1/skus/*       # Ant 的 * 只匹配单段，/skus/{id}/price、/stock 仍需 JWT
```

> 白名单分支会先清除客户端可能伪造的 `X-User-Id` / `X-User-Role`（以及历史头 `uid`）再放行，
> 因此**不要期望在白名单接口里读到用户身份**；需要用户身份的接口必须走鉴权。
>
> 经网关访问时代码里的 `X-User-Id` / `X-User-Role` 由网关注入（网关仅注入这两个头，
> 不再下发非标准头 `uid`；uid 固定取 JWT 标准 claim `sub`），服务本身不做 JWT 校验——
> 直连 8081 会绕过鉴权，生产环境应保证该端口不对外暴露。
