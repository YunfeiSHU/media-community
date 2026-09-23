package com.example.product;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.example.product.api.ProductRpcService;
import com.example.product.api.StockRpcService;
import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import com.example.product.api.dto.StockDTO;
import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;
import com.example.product.domain.enums.ProductStatus;
import com.example.product.domain.enums.ProductType;
import com.example.product.domain.enums.SkuStatus;
import com.example.product.dto.request.CreateProductRequest;
import com.example.product.dto.request.CreateSkuRequest;
import com.example.product.dto.request.UpdatePriceRequest;
import com.example.product.dto.request.UpdateProductRequest;
import com.example.product.dto.request.UpdateSkuRequest;
import com.example.product.dto.response.PriceVO;
import com.example.product.dto.response.ProductDetailVO;
import com.example.product.dto.response.SkuVO;
import com.example.product.dto.response.StockVO;
import com.example.product.service.PriceService;
import com.example.product.service.ProductService;
import com.example.product.service.SkuService;
import com.example.product.service.StockService;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.transaction.annotation.Transactional;

/**
 * 领域服务 + RPC 服务集成测试（H2 内存库，方法级事务回滚）。
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
@Transactional
class ProductDomainIntegrationTest {

    private static final String UID = "u-1001";

    @Autowired
    private ProductService productService;

    @Autowired
    private SkuService skuService;

    @Autowired
    private PriceService priceService;

    @Autowired
    private StockService stockService;

    @Autowired
    private ProductRpcService productRpcService;

    @Autowired
    private StockRpcService stockRpcService;

    @Test
    @DisplayName("商品：创建 → 查询 → 修改 → 删除")
    void productCrud() {
        ProductDetailVO created = createProduct("测试商品A");
        String productId = created.product().productId();
        assertThat(productId).startsWith("P");
        assertThat(created.product().status()).isEqualTo(ProductStatus.OFF_SHELF.getCode());
        assertThat(created.skus()).isEmpty();

        // 业务编号 / 内部ID 均可查询
        assertThat(productService.getProduct(productId).product().name()).isEqualTo("测试商品A");
        assertThat(productService.getProduct(String.valueOf(created.product().id())).product().name()).isEqualTo("测试商品A");

        ProductDetailVO updated = productService.updateProduct(productId,
                new UpdateProductRequest("测试商品A-改", "新描述", null, ProductType.VIRTUAL.getCode()));
        assertThat(updated.product().name()).isEqualTo("测试商品A-改");
        assertThat(updated.product().description()).isEqualTo("新描述");
        assertThat(updated.product().productType()).isEqualTo(ProductType.VIRTUAL.getCode());

        productService.deleteProduct(productId);
        assertThatThrownBy(() -> productService.getProduct(productId))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.PRODUCT_NOT_FOUND);
    }

    @Test
    @DisplayName("商品：没有启用SKU时不允许上架；有SKU后可上下架")
    void productOnShelfRequiresEnabledSku() {
        String productId = createProduct("上架测试").product().productId();

        assertThatThrownBy(() -> productService.onShelf(productId))
                .isInstanceOf(BizException.class)
                .hasMessageContaining("启用中的SKU");

        SkuVO sku = createSku(productId, "规格1", 9900L, 10);
        assertThat(sku.status()).isEqualTo(SkuStatus.ENABLED.getCode());

        assertThat(productService.onShelf(productId).product().status()).isEqualTo(ProductStatus.ON_SHELF.getCode());
        assertThat(productService.offShelf(productId).product().status()).isEqualTo(ProductStatus.OFF_SHELF.getCode());

        // 上架状态下不允许删除商品
        productService.onShelf(productId);
        assertThatThrownBy(() -> productService.deleteProduct(productId))
                .isInstanceOf(BizException.class)
                .hasMessageContaining("请先下架");
    }

    @Test
    @DisplayName("SKU：创建 / 查询 / 修改 / 删除")
    void skuCrud() {
        String productId = createProduct("SKU测试").product().productId();
        SkuVO sku = createSku(productId, "红色-M", 12900L, 20);
        assertThat(sku.skuId()).startsWith("S");
        assertThat(sku.availableStock()).isEqualTo(20);
        assertThat(sku.lockedStock()).isZero();
        assertThat(sku.productId()).isNotNull();

        assertThat(skuService.getSku(sku.skuId()).name()).isEqualTo("红色-M");
        assertThat(skuService.getSku(String.valueOf(sku.id())).skuId()).isEqualTo(sku.skuId());

        SkuVO updated = skuService.updateSku(sku.skuId(),
                new UpdateSkuRequest("红色-L", SkuStatus.DISABLED.getCode()));
        assertThat(updated.name()).isEqualTo("红色-L");
        assertThat(updated.status()).isEqualTo(SkuStatus.DISABLED.getCode());

        skuService.deleteSku(sku.skuId());
        assertThatThrownBy(() -> skuService.getSku(sku.skuId()))
                .isInstanceOf(BizException.class)
                .hasMessageContaining("SKU不存在");
    }

    @Test
    @DisplayName("商品删除时级联逻辑删除其 SKU")
    void deleteProductCascadeSku() {
        String productId = createProduct("级联删除").product().productId();
        SkuVO sku = createSku(productId, "规格A", 100L, 1);

        productService.deleteProduct(productId);

        assertThatThrownBy(() -> skuService.getSku(sku.skuId())).isInstanceOf(BizException.class);
        assertThatThrownBy(() -> stockService.getStock(sku.skuId())).isInstanceOf(BizException.class);
    }

    @Test
    @DisplayName("价格：当前价格查询与修改")
    void priceQueryAndUpdate() {
        String productId = createProduct("价格测试").product().productId();
        SkuVO sku = createSku(productId, "规格A", 9900L, 1);

        PriceVO current = priceService.getCurrentPrice(sku.skuId());
        assertThat(current.price()).isEqualTo(9900L);
        assertThat(current.currency()).isEqualTo("CNY");

        PriceVO updated = priceService.updatePrice(sku.skuId(), new UpdatePriceRequest(12800L));
        assertThat(updated.price()).isEqualTo(12800L);
        assertThat(priceService.getCurrentPrice(sku.skuId()).price()).isEqualTo(12800L);
    }

    @Test
    @DisplayName("库存：增加 / 扣减 / 释放 / 确认 与库存不足")
    void stockFlow() {
        String productId = createProduct("库存测试").product().productId();
        SkuVO sku = createSku(productId, "规格A", 100L, 10);

        StockVO increased = stockService.increaseStock(sku.skuId(), 5);
        assertThat(increased.availableStock()).isEqualTo(15);

        StockVO reserved = stockService.reserveStock(sku.skuId(), 10);
        assertThat(reserved.availableStock()).isEqualTo(5);
        assertThat(reserved.lockedStock()).isEqualTo(10);

        StockVO released = stockService.releaseStock(sku.skuId(), 4);
        assertThat(released.availableStock()).isEqualTo(9);
        assertThat(released.lockedStock()).isEqualTo(6);

        StockVO confirmed = stockService.confirmStock(sku.skuId(), 6);
        assertThat(confirmed.availableStock()).isEqualTo(9);
        assertThat(confirmed.lockedStock()).isZero();

        // 可用库存不足：扣减失败且数据保持不变
        assertThatThrownBy(() -> stockService.reserveStock(sku.skuId(), 10))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.STOCK_NOT_ENOUGH);
        assertThat(stockService.getStock(sku.skuId()).availableStock()).isEqualTo(9);

        // 锁定库存不足：确认失败
        assertThatThrownBy(() -> stockService.confirmStock(sku.skuId(), 1))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.STOCK_LOCK_NOT_ENOUGH);
    }

    @Test
    @DisplayName("RPC：ProductRpcService / StockRpcService")
    void rpcServices() {
        String productId = createProduct("RPC测试").product().productId();
        SkuVO sku = createSku(productId, "规格A", 6600L, 10);

        ProductDTO product = productRpcService.getProduct(productId);
        assertThat(product).isNotNull();
        assertThat(product.getName()).isEqualTo("RPC测试");

        SkuDTO skuDTO = productRpcService.getSku(sku.skuId());
        assertThat(skuDTO).isNotNull();
        assertThat(skuDTO.getProductId()).isEqualTo(productId);
        assertThat(skuDTO.getPrice()).isEqualTo(6600L);

        assertThat(productRpcService.getSkus(productId)).hasSize(1);

        assertThat(stockRpcService.getStock(sku.skuId()).getAvailableStock()).isEqualTo(10);
        assertThat(stockRpcService.reserveStock(sku.skuId(), 3)).isTrue();

        StockDTO afterReserve = stockRpcService.getStock(sku.skuId());
        assertThat(afterReserve.getAvailableStock()).isEqualTo(7);
        assertThat(afterReserve.getLockedStock()).isEqualTo(3);

        assertThat(stockRpcService.releaseStock(sku.skuId(), 1)).isTrue();
        assertThat(stockRpcService.confirmStock(sku.skuId(), 5)).isFalse();
        assertThat(stockRpcService.reserveStock(sku.skuId(), 999)).isFalse();
        assertThat(stockRpcService.getStock("NOT_EXIST_SKU")).isNull();
        assertThat(productRpcService.getProduct("NOT_EXIST_PRODUCT")).isNull();
        assertThat(productRpcService.getSkus("NOT_EXIST_PRODUCT")).isEmpty();
    }

    @Test
    @DisplayName("参数校验：非法商品类型 / 空更新")
    void invalidRequest() {
        assertThatThrownBy(() -> productService.createProduct(UID,
                new CreateProductRequest("商品", null, null, 99)))
                .isInstanceOf(BizException.class)
                .hasMessageContaining("非法的商品类型");

        String productId = createProduct("校验测试").product().productId();
        assertThatThrownBy(() -> productService.updateProduct(productId,
                new UpdateProductRequest(null, null, null, null)))
                .isInstanceOf(BizException.class)
                .hasMessageContaining("没有需要更新的字段");
    }

    @Test
    @DisplayName("创建商品：身份缺失 / 超长 uid 被拒绝")
    void createProductRequiresValidUid() {
        CreateProductRequest request = new CreateProductRequest("身份商品", null, null, ProductType.PHYSICAL.getCode());

        assertThatThrownBy(() -> productService.createProduct(null, request))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.UNAUTHORIZED);

        assertThatThrownBy(() -> productService.createProduct("  ", request))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.UNAUTHORIZED);

        // 超过 product.uid 列宽 VARCHAR(32)
        assertThatThrownBy(() -> productService.createProduct("u".repeat(33), request))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.INVALID_PARAM);
    }

    private ProductDetailVO createProduct(String name) {
        return productService.createProduct(UID, new CreateProductRequest(
                name, "商品描述", "https://cdn.example.com/cover.png", ProductType.PHYSICAL.getCode()));
    }

    private SkuVO createSku(String productId, String name, Long price, Integer stock) {
        return skuService.createSku(productId, new CreateSkuRequest(name, price, stock));
    }
}
