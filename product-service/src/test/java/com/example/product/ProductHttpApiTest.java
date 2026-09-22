package com.example.product;

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
import org.springframework.transaction.annotation.Transactional;

/**
 * HTTP 接口契约测试：覆盖设计方案中约定的全部 Restful 路由，并校验统一响应结构与错误码。
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
@Transactional
class ProductHttpApiTest {

    private static final String JSON = MediaType.APPLICATION_JSON_VALUE;

    @Autowired
    private MockMvc mockMvc;

    @Test
    @DisplayName("HTTP：商品 / SKU / 价格 / 库存 全链路")
    void fullHttpFlow() throws Exception {
        // 1. 创建商品 POST /api/v1/products
        String productId = read(write("/api/v1/products", """
                {
                  "uid": "u-1001",
                  "name": "HTTP商品",
                  "description": "描述",
                  "coverUrl": "https://cdn.example.com/cover.png",
                  "productType": 1
                }
                """, 200, "$.code", 0), "$.data.product.productId");

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
        mockMvc.perform(put("/api/v1/skus/{id}", skuId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name": "规格-升级版", "status": 1}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.name").value("规格-升级版"));

        // 6. 修改价格 PUT /api/v1/skus/{id}/price
        mockMvc.perform(put("/api/v1/skus/{id}/price", skuId)
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
        mockMvc.perform(post("/api/v1/skus/{id}/stock/increase", skuId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 15}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(20));

        // 10. 库存扣减 POST /api/v1/skus/{id}/stock/reserve
        mockMvc.perform(post("/api/v1/skus/{id}/stock/reserve", skuId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 8}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(12))
                .andExpect(jsonPath("$.data.lockedStock").value(8));

        // 11. 库存释放 POST /api/v1/skus/{id}/stock/release
        mockMvc.perform(post("/api/v1/skus/{id}/stock/release", skuId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 8}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.availableStock").value(20))
                .andExpect(jsonPath("$.data.lockedStock").value(0));

        // 12. 上架 POST /api/v1/products/{id}/on-shelf
        mockMvc.perform(post("/api/v1/products/{id}/on-shelf", productId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.product.status").value(1));

        // 13. 修改商品 PUT /api/v1/products/{id}
        mockMvc.perform(put("/api/v1/products/{id}", productId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name": "HTTP商品-改名", "productType": 2}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.product.name").value("HTTP商品-改名"))
                .andExpect(jsonPath("$.data.skus[0].skuId").value(skuId));

        // 14. 下架 POST /api/v1/products/{id}/off-shelf
        mockMvc.perform(post("/api/v1/products/{id}/off-shelf", productId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.product.status").value(0));

        // 15. 删除 SKU DELETE /api/v1/skus/{id}
        mockMvc.perform(delete("/api/v1/skus/{id}", skuId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));

        // 16. 删除商品 DELETE /api/v1/products/{id}
        mockMvc.perform(delete("/api/v1/products/{id}", productId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));
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
        mockMvc.perform(post("/api/v1/products")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"uid": "u-1", "name": "", "productType": 1}
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(40001));

        // 库存不足 → 409 / 40903
        String productId = read(write("/api/v1/products", """
                {"uid": "u-1", "name": "库存不足商品", "productType": 1}
                """, 200, "$.code", 0), "$.data.product.productId");
        String skuId = read(write("/api/v1/products/" + productId + "/skus", """
                {"name": "规格A", "price": 100, "initialStock": 1}
                """, 200, "$.code", 0), "$.data.skuId");

        mockMvc.perform(post("/api/v1/skus/{id}/stock/reserve", skuId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"quantity": 5}
                                """))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value(40903));
    }

    /** 发送写请求并按 JSON 路径断言，返回响应体 */
    private String write(String uri, String body, int httpStatus, String path, Object expected) throws Exception {
        MvcResult result = mockMvc.perform(post(uri)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(body))
                .andExpect(status().is(httpStatus))
                .andExpect(jsonPath(path).value(expected))
                .andReturn();
        return result.getResponse().getContentAsString(StandardCharsets.UTF_8);
    }

    private String read(String body, String jsonPath) {
        return JsonPath.read(body, jsonPath);
    }
}
