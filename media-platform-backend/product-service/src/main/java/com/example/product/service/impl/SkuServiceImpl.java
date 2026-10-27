package com.example.product.service.impl;

import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;
import com.example.product.common.IdGenerator;
import com.example.product.domain.entity.Inventory;
import com.example.product.domain.entity.Product;
import com.example.product.domain.entity.Sku;
import com.example.product.domain.enums.ProductStatus;
import com.example.product.domain.enums.SkuStatus;
import com.example.product.dto.request.CreateSkuRequest;
import com.example.product.dto.request.UpdateSkuRequest;
import com.example.product.dto.response.SkuVO;
import com.example.product.mapper.InventoryMapper;
import com.example.product.mapper.ProductMapper;
import com.example.product.mapper.SkuMapper;
import com.example.product.service.SkuService;
import com.example.product.support.EntityLookup;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Slf4j
@Service
@RequiredArgsConstructor
public class SkuServiceImpl implements SkuService {

    private final SkuMapper skuMapper;

    private final InventoryMapper inventoryMapper;

    private final ProductMapper productMapper;

    private final EntityLookup entityLookup;

    @Override
    @Transactional(rollbackFor = Exception.class)
    public SkuVO createSku(String productId, CreateSkuRequest request) {
        Product product = entityLookup.requireProduct(productId);

        Sku sku = new Sku();
        sku.setSkuId(IdGenerator.nextSkuId());
        sku.setProductId(product.getId());
        sku.setName(request.name().trim());
        sku.setPrice(request.price());
        sku.setStatus(SkuStatus.ENABLED.getCode());
        skuMapper.insert(sku);

        // SKU 与库存 1:1，创建 SKU 时同步落一条库存记录
        Inventory inventory = new Inventory();
        inventory.setSkuId(sku.getId());
        inventory.setAvailableStock(request.initialStockOrDefault());
        inventory.setLockedStock(0);
        inventoryMapper.insert(inventory);

        log.info("创建SKU成功: skuId={}, productId={}, price={}, initialStock={}",
                sku.getSkuId(), product.getProductId(), sku.getPrice(), inventory.getAvailableStock());
        return SkuVO.from(skuMapper.selectById(sku.getId()), inventory);
    }

    @Override
    public SkuVO getSku(String id) {
        Sku sku = entityLookup.requireSku(id);
        log.debug("查询SKU详情: id={}, skuId={}", id, sku.getSkuId());
        return SkuVO.from(sku, inventoryMapper.selectBySkuId(sku.getId()));
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public SkuVO updateSku(String id, UpdateSkuRequest request) {
        if (request.isEmpty()) {
            throw new BizException(ErrorCode.INVALID_PARAM, "没有需要更新的字段");
        }
        Sku existing = entityLookup.requireSku(id);

        Sku update = new Sku();
        update.setId(existing.getId());
        if (request.name() != null) {
            String name = request.name().trim();
            if (name.isEmpty()) {
                throw new BizException(ErrorCode.INVALID_PARAM, "SKU名称不能为空");
            }
            update.setName(name);
        }
        if (request.status() != null) {
            // 只允许在「禁用 / 启用」之间切换，删除走 DELETE 接口
            if (SkuStatus.DELETED.getCode() == request.status()) {
                throw new BizException(ErrorCode.INVALID_PARAM, "删除请调用 DELETE 接口");
            }
            update.setStatus(request.status());
        }
        skuMapper.update(update);

        log.info("更新SKU成功: skuId={}, name={}, status={}", existing.getSkuId(), update.getName(), update.getStatus());
        return getSku(id);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public void deleteSku(String id) {
        Sku sku = entityLookup.requireSku(id);

        Product product = productMapper.selectById(sku.getProductId());
        if (product != null && ProductStatus.ON_SHELF.is(product.getStatus())) {
            throw new BizException(ErrorCode.SKU_STATUS_CONFLICT, "商品处于上架状态，请先下架商品再删除SKU");
        }
        Inventory inventory = inventoryMapper.selectBySkuId(sku.getId());
        if (inventory != null && inventory.getLockedStock() != null && inventory.getLockedStock() > 0) {
            throw new BizException(ErrorCode.SKU_STATUS_CONFLICT, "SKU存在订单锁定的库存，无法删除");
        }
        skuMapper.updateStatusById(sku.getId(), SkuStatus.DELETED.getCode());
        log.info("删除SKU成功: skuId={}, productId={}", sku.getSkuId(), sku.getProductId());
    }
}
