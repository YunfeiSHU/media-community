package com.example.product.service.impl;

import com.example.product.domain.entity.Sku;
import com.example.product.dto.request.UpdatePriceRequest;
import com.example.product.dto.response.PriceVO;
import com.example.product.mapper.SkuMapper;
import com.example.product.service.PriceService;
import com.example.product.support.EntityLookup;
import java.util.Objects;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Slf4j
@Service
@RequiredArgsConstructor
public class PriceServiceImpl implements PriceService {

    private final SkuMapper skuMapper;

    private final EntityLookup entityLookup;

    @Override
    public PriceVO getCurrentPrice(String skuId) {
        Sku sku = entityLookup.requireSku(skuId);
        log.debug("查询当前价格: skuId={}, price={}", sku.getSkuId(), sku.getPrice());
        return toPriceVO(sku);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public PriceVO updatePrice(String skuId, UpdatePriceRequest request) {
        Sku sku = entityLookup.requireSku(skuId);
        if (Objects.equals(sku.getPrice(), request.price())) {
            // 幂等：价格未变化直接返回
            log.debug("修改价格跳过（价格未变化）: skuId={}, price={}", sku.getSkuId(), request.price());
            return toPriceVO(sku);
        }
        skuMapper.updatePriceById(sku.getId(), request.price());
        log.info("修改价格成功: skuId={}, {} -> {}", sku.getSkuId(), sku.getPrice(), request.price());
        return toPriceVO(skuMapper.selectById(sku.getId()));
    }

    private PriceVO toPriceVO(Sku sku) {
        return new PriceVO(sku.getSkuId(), sku.getPrice(), PriceVO.CURRENCY_CNY, sku.getUpdatedAt());
    }
}
