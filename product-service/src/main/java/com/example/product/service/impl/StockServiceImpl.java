package com.example.product.service.impl;

import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;
import com.example.product.domain.entity.Inventory;
import com.example.product.domain.entity.Sku;
import com.example.product.dto.response.StockVO;
import com.example.product.mapper.InventoryMapper;
import com.example.product.service.StockService;
import com.example.product.support.EntityLookup;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Slf4j
@Service
@RequiredArgsConstructor
public class StockServiceImpl implements StockService {

    private final InventoryMapper inventoryMapper;

    private final EntityLookup entityLookup;

    @Override
    public StockVO getStock(String skuId) {
        Sku sku = entityLookup.requireSku(skuId);
        log.debug("查询库存: skuId={}", sku.getSkuId());
        return StockVO.from(sku.getSkuId(), requireInventory(sku.getId()));
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public StockVO increaseStock(String skuId, int quantity) {
        Sku sku = prepare(skuId, quantity);
        inventoryMapper.increaseAvailable(sku.getId(), quantity);
        log.info("增加库存成功: skuId={}, quantity={}", sku.getSkuId(), quantity);
        return reload(sku);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public StockVO reserveStock(String skuId, int quantity) {
        Sku sku = prepare(skuId, quantity);
        // 条件 UPDATE 由数据库保证并发安全，不会超卖
        if (inventoryMapper.reserve(sku.getId(), quantity) == 0) {
            log.warn("预占库存失败（可用不足）: skuId={}, quantity={}, available={}",
                    sku.getSkuId(), quantity, currentAvailable(sku.getId()));
            throw new BizException(ErrorCode.STOCK_NOT_ENOUGH,
                    "可用库存不足，当前可用 " + currentAvailable(sku.getId()) + "，本次需要 " + quantity);
        }
        log.info("预占库存成功（可用→锁定）: skuId={}, quantity={}", sku.getSkuId(), quantity);
        return reload(sku);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public StockVO releaseStock(String skuId, int quantity) {
        Sku sku = prepare(skuId, quantity);
        if (inventoryMapper.release(sku.getId(), quantity) == 0) {
            log.warn("释放库存失败（锁定不足）: skuId={}, quantity={}, locked={}",
                    sku.getSkuId(), quantity, currentLocked(sku.getId()));
            throw new BizException(ErrorCode.STOCK_LOCK_NOT_ENOUGH,
                    "锁定库存不足，当前锁定 " + currentLocked(sku.getId()) + "，本次需要释放 " + quantity);
        }
        log.info("释放库存成功（锁定→可用）: skuId={}, quantity={}", sku.getSkuId(), quantity);
        return reload(sku);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public StockVO confirmStock(String skuId, int quantity) {
        Sku sku = prepare(skuId, quantity);
        if (inventoryMapper.confirm(sku.getId(), quantity) == 0) {
            log.warn("确认扣减失败（锁定不足）: skuId={}, quantity={}, locked={}",
                    sku.getSkuId(), quantity, currentLocked(sku.getId()));
            throw new BizException(ErrorCode.STOCK_LOCK_NOT_ENOUGH,
                    "锁定库存不足，当前锁定 " + currentLocked(sku.getId()) + "，本次需要确认 " + quantity);
        }
        log.info("确认扣减库存成功（锁定→消耗）: skuId={}, quantity={}", sku.getSkuId(), quantity);
        return reload(sku);
    }

    private Sku prepare(String skuId, int quantity) {
        if (quantity <= 0) {
            throw new BizException(ErrorCode.INVALID_PARAM, "库存数量必须大于0");
        }
        Sku sku = entityLookup.requireSku(skuId);
        requireInventory(sku.getId());
        return sku;
    }

    private Inventory requireInventory(Long skuInternalId) {
        Inventory inventory = inventoryMapper.selectBySkuId(skuInternalId);
        if (inventory == null) {
            throw new BizException(ErrorCode.INVENTORY_NOT_FOUND, "SKU库存记录不存在");
        }
        return inventory;
    }

    private int currentAvailable(Long skuInternalId) {
        Inventory inventory = inventoryMapper.selectBySkuId(skuInternalId);
        return inventory == null || inventory.getAvailableStock() == null ? 0 : inventory.getAvailableStock();
    }

    private int currentLocked(Long skuInternalId) {
        Inventory inventory = inventoryMapper.selectBySkuId(skuInternalId);
        return inventory == null || inventory.getLockedStock() == null ? 0 : inventory.getLockedStock();
    }

    private StockVO reload(Sku sku) {
        return StockVO.from(sku.getSkuId(), requireInventory(sku.getId()));
    }
}
