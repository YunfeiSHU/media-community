package com.example.product;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.jayway.jsonpath.JsonPath;
import java.nio.charset.StandardCharsets;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.springframework.test.web.servlet.request.MockHttpServletRequestBuilder;
import org.springframework.transaction.annotation.Transactional;

/**
 * HTTP 接口契约测试：覆盖设计方案中约定的全部 Restful 路由，并校验统一响应结构与错误码。
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
@Transactional
class ProductHttpApiTest {

    /** 网关注入的身份头（值 = JWT 的 sub） */
    private static final String UID_HEADER = "X-User-Id";

    private static final String UID = "u-1001";

    @Autowired
    private MockMvc mockMvc;

    @Test
    @DisplayName("HTTP：商品 / SKU / 价格 / 库存 全链路")
    void fullHttpFlow() throws Exception {
        // 1. 创建商品 POST /api/v1/products（创建者取网关注入的 X-User-Id）
        String created = write("/api/v1/products", """
                {
                  "name": "HTTP商品",
                  "description": "描述",
                  "coverUrl": "https://cdn.example.com/cover.png",
                  "productType": 1
                }
                """, 200, "$.code", 0);
        assertThat(read(created, "$.data.product.uid")).isEqualTo(UID);
        String productId = read(created, "$.data.product.productId");

        // 2. 商品详情 GET /api/v1/products/{id}
        mockMvc.perform(get("/api/v1/products/{id}", productId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.product.productId").value(productId))
                .andExpect(jsonPath("$.data.product.status").value(0))
                .andExpect(jsonPath("$.data.skus").isEmpty());

        // 3. 创建 SKU POST /api/v1/products/{id}/skus
        String skuId = read(write("/api/v1/products/" + productId + "/skus", """
                {"name": "规格-默认", "price": 9900, "initialStock": 5}
                """, 200, "$.code", 0), "$.data.skuId");

        // 4. SKU详情 GET /api/v1/skus/{id}
        mockMvc.perform(get("/api/v1/skus/{id}", skuId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.skuId").value(skuId))
                .andExpect(jsonPath("$.data.price").value(9900))
                .andExpect(jsonPath("$.data.availableStock").value(5));

        // 5. 修改 SKU PUT /api/v1/skus/{id}
        mockMvc.perform(asUser(put("/api/v1/skus/{id}", skuId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name": "规格-升级版", "status": 1}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.name").value("规格-升级版"));

        // 6. 修改价格 PUT /api/v1/skus/{id}/price
        mockMvc.perform(asUser(put("/api/v1/skus/{id}/price", skuId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"price": 15800}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.price").value(15800))
                .andExpect(jsonPath("$.data.currency").value("CNY"));

        // 7. 当前价格查询 GET /api/v1/skus/{id}/price
        mockMvc.perform(get("/api/v1/skus/{id}/price", skuId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.price").value(15800));

        // 8. 查询库存 GET /api/v1/skus/{id}/stock
        mockMvc.perform(get("/api/v1/skus/{id}/stock", skuId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(5))
                .andExpect(jsonPath("$.data.lockedStock").value(0));

        // 9. 增加库存 POST /api/v1/skus/{id}/stock/increase
        mockMvc.perform(asUser(post("/api/v1/skus/{id}/stock/increase", skuId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 15}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(20));

        // 10. 库存扣减 POST /api/v1/skus/{id}/stock/reserve
        mockMvc.perform(asUser(post("/api/v1/skus/{id}/stock/reserve", skuId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 8}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(12))
                .andExpect(jsonPath("$.data.lockedStock").value(8));

        // 11. 库存释放 POST /api/v1/skus/{id}/stock/release
        mockMvc.perform(asUser(post("/api/v1/skus/{id}/stock/release", skuId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 8}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(20))
                .andExpect(jsonPath("$.data.lockedStock").value(0));

        // 12. 上架 POST /api/v1/products/{id}/on-shelf
        mockMvc.perform(asUser(post("/api/v1/products/{id}/on-shelf", productId)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.product.status").value(1));

        // 13. 修改商品 PUT /api/v1/products/{id}
        mockMvc.perform(asUser(put("/api/v1/products/{id}", productId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name": "HTTP商品-改名", "productType": 2}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.product.name").value("HTTP商品-改名"))
                .andExpect(jsonPath("$.data.skus[0].skuId").value(skuId));

        // 14. 下架 POST /api/v1/products/{id}/off-shelf
        mockMvc.perform(asUser(post("/api/v1/products/{id}/off-shelf", productId)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.product.status").value(0));

        // 15. 删除 SKU DELETE /api/v1/skus/{id}
        mockMvc.perform(asUser(delete("/api/v1/skus/{id}", skuId)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));

        // 16. 删除商品 DELETE /api/v1/products/{id}
        mockMvc.perform(asUser(delete("/api/v1/products/{id}", productId)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));
    }

    @Test
    @DisplayName("HTTP：写接口只允许商品创建者操作，他人一律 403 / 40301")
    void writeEndpointsRejectNonOwner() throws Exception {
        String created = write("/api/v1/products", """
                {"name": "归属校验商品", "productType": 1}
                """, 200, "$.code", 0);
        String productId = read(created, "$.data.product.productId");
        String skuId = read(write("/api/v1/products/" + productId + "/skus", """
                {"name": "归属SKU", "price": 100, "initialStock": 3}
                """, 200, "$.code", 0), "$.data.skuId");

        String other = "u-9999";
        // 商品级写操作
        assertForbidden(asUser(other, put("/api/v1/products/{id}", productId))
                .contentType(MediaType.APPLICATION_JSON).content("{\"name\":\"改名\"}"));
        assertForbidden(asUser(other, delete("/api/v1/products/{id}", productId)));
        assertForbidden(asUser(other, post("/api/v1/products/{id}/on-shelf", productId)));
        assertForbidden(asUser(other, post("/api/v1/products/{id}/off-shelf", productId)));
        assertForbidden(asUser(other, post("/api/v1/products/{id}/skus", productId))
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"name\":\"越权SKU\",\"price\":1,\"initialStock\":1}"));
        // SKU 级写操作
        assertForbidden(asUser(other, put("/api/v1/skus/{id}", skuId))
                .contentType(MediaType.APPLICATION_JSON).content("{\"name\":\"改名\"}"));
        assertForbidden(asUser(other, delete("/api/v1/skus/{id}", skuId)));
        assertForbidden(asUser(other, put("/api/v1/skus/{id}/price", skuId))
                .contentType(MediaType.APPLICATION_JSON).content("{\"price\":1}"));
        assertForbidden(asUser(other, post("/api/v1/skus/{id}/stock/increase", skuId))
                .contentType(MediaType.APPLICATION_JSON).content("{\"quantity\":1}"));
        assertForbidden(asUser(other, post("/api/v1/skus/{id}/stock/reserve", skuId))
                .contentType(MediaType.APPLICATION_JSON).content("{\"quantity\":1}"));
        assertForbidden(asUser(other, post("/api/v1/skus/{id}/stock/release", skuId))
                .contentType(MediaType.APPLICATION_JSON).content("{\"quantity\":1}"));
        assertForbidden(asUser(other, post("/api/v1/skus/{id}/stock/confirm", skuId))
                .contentType(MediaType.APPLICATION_JSON).content("{\"quantity\":1}"));

        // 被拒绝后数据未被改动（价格 / 库存仍为创建时的值）
        mockMvc.perform(get("/api/v1/skus/{id}/price", skuId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.price").value(100));
        mockMvc.perform(get("/api/v1/skus/{id}/stock", skuId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(3));

        // 只读接口对他人依旧开放
        mockMvc.perform(get("/api/v1/products/{id}", productId)).andExpect(status().isOk());
        mockMvc.perform(get("/api/v1/skus/{id}", skuId)).andExpect(status().isOk());

        // 创建者本人仍可正常操作
        mockMvc.perform(asUser(put("/api/v1/skus/{id}/price", skuId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"price\":250}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.price").value(250));
    }

    private void assertForbidden(MockHttpServletRequestBuilder builder) throws Exception {
        mockMvc.perform(builder)
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.code").value(40301));
    }

    @Test
    @DisplayName("HTTP：错误码与状态码映射")
    void errorResponses() throws Exception {
        // 商品不存在 → 404 / 40401
        mockMvc.perform(get("/api/v1/products/{id}", "999999"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value(40401));

        // SKU不存在 → 404 / 40402
        mockMvc.perform(get("/api/v1/skus/{id}/stock", "NOT_EXIST_SKU"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value(40402));

        // 参数校验失败 → 400 / 40001
        mockMvc.perform(asUser(post("/api/v1/products"))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name": "", "productType": 1}
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(40001));

        // 库存不足 → 409 / 40903
        String productId = read(write("/api/v1/products", """
                {"name": "库存不足商品", "productType": 1}
                """, 200, "$.code", 0), "$.data.product.productId");
        String skuId = read(write("/api/v1/products/" + productId + "/skus", """
                {"name": "规格A", "price": 100, "initialStock": 1}
                """, 200, "$.code", 0), "$.data.skuId");

        mockMvc.perform(asUser(post("/api/v1/skus/{id}/stock/reserve", skuId))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 5}
                                """))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value(40903));
    }

    @Test
    @DisplayName("HTTP：创建商品只认网关注入的 X-User-Id，请求体里的 uid 被忽略")
    void createProductUsesGatewayIdentity() throws Exception {
        // 请求体里的 uid 不再参与写入（未知字段被 Jackson 忽略）
        mockMvc.perform(asUser(post("/api/v1/products"))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"uid": "forged-uid", "name": "身份测试商品", "productType": 1}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.product.uid").value(UID));

        // 缺少身份头（例如绕过网关直连）→ 401 / 40101
        mockMvc.perform(post("/api/v1/products")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name": "无身份商品", "productType": 1}
                                """))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value(40101));
    }

    /** 发送写请求（带网关注入的身份头）并按 JSON 路径断言，返回响应体 */
    private String write(String uri, String body, int httpStatus, String path, Object expected) throws Exception {
        MvcResult result = mockMvc.perform(asUser(post(uri))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(body))
                .andExpect(status().is(httpStatus))
                .andExpect(jsonPath(path).value(expected))
                .andReturn();
        return result.getResponse().getContentAsString(StandardCharsets.UTF_8);
    }

    /** 以商品创建者身份发起请求（等价于网关校验 JWT 后注入 X-User-Id） */
    private MockHttpServletRequestBuilder asUser(MockHttpServletRequestBuilder builder) {
        return asUser(UID, builder);
    }

    /** 以指定用户身份发起请求 */
    private MockHttpServletRequestBuilder asUser(String uid, MockHttpServletRequestBuilder builder) {
        return builder.header(UID_HEADER, uid);
    }

    private String read(String body, String jsonPath) {
        return JsonPath.read(body, jsonPath);
    }
}
