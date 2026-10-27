package com.example.product.service.impl;

import com.example.product.common.BizException;
import com.example.product.common.CurrentUser;
import com.example.product.common.ErrorCode;
import com.example.product.common.IdGenerator;
import com.example.product.common.PageResult;
import com.example.product.domain.entity.Inventory;
import com.example.product.domain.entity.Product;
import com.example.product.domain.entity.Sku;
import com.example.product.domain.enums.ProductStatus;
import com.example.product.domain.enums.ProductType;
import com.example.product.domain.enums.SkuStatus;
import com.example.product.dto.request.CreateProductRequest;
import com.example.product.dto.request.UpdateProductRequest;
import com.example.product.dto.response.ProductDetailVO;
import com.example.product.dto.response.ProductSkuStat;
import com.example.product.dto.response.ProductVO;
import com.example.product.mapper.InventoryMapper;
import com.example.product.mapper.ProductMapper;
import com.example.product.mapper.SkuMapper;
import com.example.product.service.ProductService;
import com.example.product.support.EntityLookup;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Slf4j
@Service
@RequiredArgsConstructor
public class ProductServiceImpl implements ProductService {

    /** 列表分页上限：与《契约约定》一致，超出直接 40001，不静默截断 */
    private static final int MAX_PAGE_SIZE = 100;

    private final ProductMapper productMapper;

    private final SkuMapper skuMapper;

    private final InventoryMapper inventoryMapper;

    private final EntityLookup entityLookup;

    @Override
    @Transactional(rollbackFor = Exception.class)
    public ProductDetailVO createProduct(String uid, CreateProductRequest request) {
        // 创建者只能来自网关注入的身份（JWT sub），不接受请求体里的 uid
        String currentUid = CurrentUser.requireUid(uid);
        ProductType.of(request.productType());

        Product product = new Product();
        product.setProductId(IdGenerator.nextProductId());
        product.setUid(currentUid);
        product.setName(request.name().trim());
        product.setDescription(request.description());
        product.setCoverUrl(request.coverUrl());
        product.setProductType(request.productType());
        // 新商品默认下架，待 SKU 配置完成后再上架
        product.setStatus(ProductStatus.OFF_SHELF.getCode());
        productMapper.insert(product);

        log.info("创建商品成功: productId={}, uid={}, name={}, productType={}",
                product.getProductId(), product.getUid(), product.getName(), product.getProductType());
        return loadDetail(entityLookup.requireProduct(product.getProductId()));
    }

    @Override
    public ProductDetailVO getProduct(String id) {
        Product product = entityLookup.requireProduct(id);
        log.debug("查询商品详情: id={}, productId={}", id, product.getProductId());
        return loadDetail(product);
    }

    @Override
    public PageResult<ProductVO> listProducts(String keyword, Integer status, int page, int pageSize) {
        return listInternal(keyword, status, null, page, pageSize);
    }

    @Override
    public PageResult<ProductVO> listMyProducts(String uid, int page, int pageSize) {
        if (uid == null || uid.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少登录用户");
        }
        return listInternal(null, null, uid, page, pageSize);
    }

    private PageResult<ProductVO> listInternal(String keyword, Integer status, String uid, int page, int pageSize) {
        if (page < 1) {
            throw new BizException(ErrorCode.INVALID_PARAM, "page 必须从 1 开始");
        }
        if (pageSize < 1 || pageSize > MAX_PAGE_SIZE) {
            throw new BizException(ErrorCode.INVALID_PARAM, "pageSize 必须在 1~" + MAX_PAGE_SIZE + " 之间");
        }

        String normalizedKeyword = keyword == null ? null : keyword.trim();
        long total = productMapper.countPage(normalizedKeyword, status, uid);
        if (total == 0) {
            return PageResult.empty(page, pageSize);
        }

        List<Product> products = productMapper.selectPage(
                normalizedKeyword, status, uid, (page - 1) * pageSize, pageSize);
        if (products.isEmpty()) {
            return PageResult.of(List.of(), total, page, pageSize);
        }

        // 一页商品一次聚合，避免逐个商品查 SKU（N+1）
        List<Long> ids = products.stream().map(Product::getId).toList();
        Map<Long, ProductSkuStat> stats = skuMapper.selectStatsByProductIds(ids).stream()
                .collect(Collectors.toMap(ProductSkuStat::getProductId, Function.identity()));

        List<ProductVO> items = products.stream()
                .map(product -> ProductVO.from(product, stats.get(product.getId())))
                .toList();
        return PageResult.of(items, total, page, pageSize);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public ProductDetailVO updateProduct(String id, UpdateProductRequest request) {
        if (request.isEmpty()) {
            throw new BizException(ErrorCode.INVALID_PARAM, "没有需要更新的字段");
        }
        Product existing = entityLookup.requireProduct(id);

        Product update = new Product();
        update.setId(existing.getId());
        if (request.name() != null) {
            String name = request.name().trim();
            if (name.isEmpty()) {
                throw new BizException(ErrorCode.INVALID_PARAM, "商品名称不能为空");
            }
            update.setName(name);
        }
        if (request.description() != null) {
            update.setDescription(request.description());
        }
        if (request.coverUrl() != null) {
            update.setCoverUrl(request.coverUrl());
        }
        if (request.productType() != null) {
            ProductType.of(request.productType());
            update.setProductType(request.productType());
        }
        productMapper.update(update);

        log.info("更新商品成功: productId={}, name={}, productType={}",
                existing.getProductId(), update.getName(), update.getProductType());
        return loadDetail(entityLookup.requireProduct(id));
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public void deleteProduct(String id) {
        Product product = entityLookup.requireProduct(id);
        if (ProductStatus.ON_SHELF.is(product.getStatus())) {
            throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品处于上架状态，请先下架再删除");
        }
        // 带上原状态做乐观校验，避免并发下把刚上架的商品删掉
        int rows = productMapper.updateStatus(product.getId(), ProductStatus.DELETED.getCode(), product.getStatus());
        if (rows == 0) {
            throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品状态已变更，请刷新后重试");
        }
        skuMapper.updateStatusByProductId(product.getId(), SkuStatus.DELETED.getCode());
        log.info("删除商品成功（级联逻辑删除SKU）: productId={}", product.getProductId());
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public ProductDetailVO onShelf(String id) {
        Product product = entityLookup.requireProduct(id);
        if (ProductStatus.ON_SHELF.is(product.getStatus())) {
            throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品已处于上架状态");
        }
        if (skuMapper.countByProductIdAndStatus(product.getId(), SkuStatus.ENABLED.getCode()) == 0) {
            throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品下没有启用中的SKU，无法上架");
        }
        if (productMapper.updateStatus(product.getId(), ProductStatus.ON_SHELF.getCode(), product.getStatus()) == 0) {
            throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品状态已变更，请刷新后重试");
        }
        log.info("商品上架成功: productId={}", product.getProductId());
        return loadDetail(entityLookup.requireProduct(id));
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public ProductDetailVO offShelf(String id) {
        Product product = entityLookup.requireProduct(id);
        if (ProductStatus.OFF_SHELF.is(product.getStatus())) {
            throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品已处于下架状态");
        }
        if (productMapper.updateStatus(product.getId(), ProductStatus.OFF_SHELF.getCode(), product.getStatus()) == 0) {
            throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品状态已变更，请刷新后重试");
        }
        log.info("商品下架成功: productId={}", product.getProductId());
        return loadDetail(entityLookup.requireProduct(id));
    }

    /** 组装商品详情：一次批量查询库存，避免 N+1 */
    private ProductDetailVO loadDetail(Product product) {
        List<Sku> skus = skuMapper.selectByProductId(product.getId());
        Map<Long, Inventory> inventoryMap = skus.isEmpty()
                ? Map.of()
                : inventoryMapper.selectBySkuIds(skus.stream().map(Sku::getId).toList()).stream()
                        .collect(Collectors.toMap(Inventory::getSkuId, Function.identity()));
        return ProductDetailVO.of(product, skus, inventoryMap);
    }
}
