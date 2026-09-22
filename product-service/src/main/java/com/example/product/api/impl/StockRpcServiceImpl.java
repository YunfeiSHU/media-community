package com.example.product.api.impl;

import com.example.product.api.StockRpcService;
import com.example.product.api.dto.StockDTO;
import com.example.product.common.BizException;
import com.example.product.domain.entity.Inventory;
import com.example.product.domain.entity.Sku;
import com.example.product.domain.enums.SkuStatus;
import com.example.product.mapper.InventoryMapper;
import com.example.product.service.StockService;
import com.example.product.support.EntityLookup;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.apache.dubbo.config.annotation.DubboService;
import org.springframework.transaction.annotation.Transactional;

/**
 * 库存域 Dubbo 服务实现。
 */
@Slf4j
@DubboService(version = "1.0.0")
@RequiredArgsConstructor
public class StockRpcServiceImpl implements StockRpcService {

    private final StockService stockService;

    private final EntityLookup entityLookup;

    private final InventoryMapper inventoryMapper;

    @Override
    @Transactional(readOnly = true)
    public StockDTO getStock(String skuId) {
        Sku sku = entityLookup.findSku(skuId);
        if (sku == null || SkuStatus.DELETED.is(sku.getStatus())) {
            log.warn("[RPC] getStock 未命中: skuId={}", skuId);
            return null;
        }
        Inventory inventory = inventoryMapper.selectBySkuId(sku.getId());
        if (inventory == null) {
            log.warn("[RPC] getStock 库存记录缺失: skuId={}", skuId);
            return null;
        }
        log.debug("[RPC] getStock 成功: skuId={}, available={}, locked={}",
                sku.getSkuId(), inventory.getAvailableStock(), inventory.getLockedStock());
        return StockDTO.of(sku.getSkuId(), inventory);
    }

    @Override
    public boolean reserveStock(String skuId, int quantity) {
        return execute("reserveStock", skuId, quantity, () -> stockService.reserveStock(skuId, quantity));
    }

    @Override
    public boolean confirmStock(String skuId, int quantity) {
        return execute("confirmStock", skuId, quantity, () -> stockService.confirmStock(skuId, quantity));
    }

    @Override
    public boolean releaseStock(String skuId, int quantity) {
        return execute("releaseStock", skuId, quantity, () -> stockService.releaseStock(skuId, quantity));
    }

    /**
     * 统一把业务异常转换成 false，避免上游因为一次库存不足而收到异常并触发无谓重试。
     */
    private boolean execute(String action, String skuId, int quantity, Runnable operation) {
        try {
            operation.run();
            log.info("[RPC] {} 成功: skuId={}, quantity={}", action, skuId, quantity);
            return true;
        } catch (BizException ex) {
            log.warn("[RPC] {} 未生效: skuId={}, quantity={}, reason={}", action, skuId, quantity, ex.getMessage());
            return false;
        }
    }
}
