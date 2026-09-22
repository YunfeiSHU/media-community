package com.example.order.support;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.product.api.ProductRpcService;
import com.example.product.api.StockRpcService;
import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import java.util.function.Supplier;
import lombok.extern.slf4j.Slf4j;
import org.apache.dubbo.config.annotation.DubboReference;
import org.springframework.stereotype.Component;

/**
 * product-service 的 RPC 门面。
 *
 * <p>把远端的失败（无可用提供者、连接不可达、超时、序列化异常等）统一翻译成
 * {@link BizException} + {@link ErrorCode#RPC_INVOKE_FAILED}，避免基础设施异常泄漏成 500；
 * 同时把远端调用收敛到一个类里，便于测试时整体替换（{@code @MockBean}）。
 *
 * <p>调用语义：
 * <ul>
 *     <li>查询类：查不到返回 {@code null}，由业务侧判定</li>
 *     <li>库存变更类：返回 {@code false} 表示业务未生效（库存不足 / 记录不存在）</li>
 * </ul>
 */
@Slf4j
@Component
public class ProductRpcClient {

    private static final String VERSION = "1.0.0";

    /**
     * 商品域 RPC 引用。
     *
     * <p>本地联调走 {@code order.rpc.product-url} 直连 product-service（其 registry.address=N/A，
     * 不走注册中心）；切到 Nacos 注册中心时，把该注解的 {@code url} 属性删掉、并把
     * {@code dubbo.registry.address} 配成 {@code nacos://...} 即可自动发现。
     *
     * <p>{@code lazy = true}：连接推迟到首次调用，避免启动阶段就去连 provider，
     * 与 {@code check = false} 一起保证 provider 未就绪时本服务仍能正常启动。
     */
    @DubboReference(version = VERSION, check = false, lazy = true,
            url = "${order.rpc.product-url:dubbo://127.0.0.1:20880}")
    private ProductRpcService productRpcService;

    /** 库存域 RPC 引用 */
    @DubboReference(version = VERSION, check = false, lazy = true,
            url = "${order.rpc.product-url:dubbo://127.0.0.1:20880}")
    private StockRpcService stockRpcService;

    /** 查询 SKU，不存在返回 null */
    public SkuDTO getSku(String skuId) {
        return execute("getSku", () -> productRpcService.getSku(skuId));
    }

    /** 查询商品，不存在返回 null */
    public ProductDTO getProduct(String productId) {
        return execute("getProduct", () -> productRpcService.getProduct(productId));
    }

    /** 预占库存（可用库存 → 锁定库存），库存不足返回 false */
    public boolean reserveStock(String skuId, int quantity) {
        return execute("reserveStock", () -> stockRpcService.reserveStock(skuId, quantity));
    }

    /** 确认扣减（消耗锁定库存），失败返回 false */
    public boolean confirmStock(String skuId, int quantity) {
        return execute("confirmStock", () -> stockRpcService.confirmStock(skuId, quantity));
    }

    /** 释放库存（锁定库存 → 可用库存），失败返回 false */
    public boolean releaseStock(String skuId, int quantity) {
        return execute("releaseStock", () -> stockRpcService.releaseStock(skuId, quantity));
    }

    private <T> T execute(String action, Supplier<T> call) {
        try {
            return call.get();
        } catch (BizException ex) {
            // 业务异常原样抛出，避免被下面的兜底逻辑二次包装
            throw ex;
        } catch (RuntimeException ex) {
            // Dubbo 的失败形态不止 RpcException：连接不可达时（lazy 引用）会抛 IllegalStateException，
            // 这里统一兜底翻译，保证上游拿到的是 503 而不是 500
            log.error("[RPC] product-service.{} 调用失败: {}", action, ex.getMessage(), ex);
            throw new BizException(ErrorCode.RPC_INVOKE_FAILED, "调用商品服务失败: " + action);
        }
    }
}
