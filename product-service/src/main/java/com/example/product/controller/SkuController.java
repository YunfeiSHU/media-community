package com.example.product.controller;

import com.example.product.common.ApiResponse;
import com.example.product.common.CurrentUser;
import com.example.product.dto.request.StockChangeRequest;
import com.example.product.dto.request.UpdatePriceRequest;
import com.example.product.dto.request.UpdateSkuRequest;
import com.example.product.dto.response.PriceVO;
import com.example.product.dto.response.SkuVO;
import com.example.product.dto.response.StockVO;
import com.example.product.service.PriceService;
import com.example.product.service.SkuService;
import com.example.product.service.StockService;
import com.example.product.support.ProductOwnership;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * SKU / 价格 / 库存相关 HTTP 接口。
 *
 * <p>写接口都要求调用者是该 SKU 所属商品的<b>创建者</b>（{@code product.uid} == 网关注入的
 * {@code X-User-Id}），否则 403 / 40301，见 {@link ProductOwnership}。
 * SKU 详情、当前价格、库存查询为只读接口，不校验归属。
 */
@RestController
@RequestMapping("/api/v1/skus")
@RequiredArgsConstructor
public class SkuController {

    private final SkuService skuService;

    private final PriceService priceService;

    private final StockService stockService;

    private final ProductOwnership productOwnership;

    /** SKU详情 GET /api/v1/skus/{id} */
    @GetMapping("/{id}")
    public ApiResponse<SkuVO> getSku(@PathVariable("id") String id) {
        return ApiResponse.success(skuService.getSku(id));
    }

    /** 修改 SKU PUT /api/v1/skus/{id}（仅商品创建者） */
    @PutMapping("/{id}")
    public ApiResponse<SkuVO> updateSku(@PathVariable("id") String id,
                                        @Valid @RequestBody UpdateSkuRequest request,
                                        HttpServletRequest httpRequest) {
        productOwnership.requireSkuOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(skuService.updateSku(id, request));
    }

    /** 删除 SKU DELETE /api/v1/skus/{id}（仅商品创建者） */
    @DeleteMapping("/{id}")
    public ApiResponse<Void> deleteSku(@PathVariable("id") String id,
                                       HttpServletRequest httpRequest) {
        productOwnership.requireSkuOwner(id, CurrentUser.uid(httpRequest));
        skuService.deleteSku(id);
        return ApiResponse.success();
    }

    /** 当前价格查询 GET /api/v1/skus/{id}/price */
    @GetMapping("/{id}/price")
    public ApiResponse<PriceVO> getPrice(@PathVariable("id") String id) {
        return ApiResponse.success(priceService.getCurrentPrice(id));
    }

    /** 修改价格 PUT /api/v1/skus/{id}/price（仅商品创建者） */
    @PutMapping("/{id}/price")
    public ApiResponse<PriceVO> updatePrice(@PathVariable("id") String id,
                                            @Valid @RequestBody UpdatePriceRequest request,
                                            HttpServletRequest httpRequest) {
        productOwnership.requireSkuOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(priceService.updatePrice(id, request));
    }

    /** 查询库存 GET /api/v1/skus/{id}/stock */
    @GetMapping("/{id}/stock")
    public ApiResponse<StockVO> getStock(@PathVariable("id") String id) {
        return ApiResponse.success(stockService.getStock(id));
    }

    /** 增加库存 POST /api/v1/skus/{id}/stock/increase（仅商品创建者） */
    @PostMapping("/{id}/stock/increase")
    public ApiResponse<StockVO> increaseStock(@PathVariable("id") String id,
                                              @Valid @RequestBody StockChangeRequest request,
                                              HttpServletRequest httpRequest) {
        productOwnership.requireSkuOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(stockService.increaseStock(id, request.quantity()));
    }

    /**
     * 库存扣减（订单预占） POST /api/v1/skus/{id}/stock/reserve。
     *
     * <p>仅供联调/排障的 HTTP 镜像接口：正常交易链路走 Dubbo（{@code StockRpcService}，无用户上下文，不做归属校验）。
     * HTTP 上仍要求调用者是商品创建者，避免任意用户改动他人库存。
     */
    @PostMapping("/{id}/stock/reserve")
    public ApiResponse<StockVO> reserveStock(@PathVariable("id") String id,
                                             @Valid @RequestBody StockChangeRequest request,
                                             HttpServletRequest httpRequest) {
        productOwnership.requireSkuOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(stockService.reserveStock(id, request.quantity()));
    }

    /** 库存释放（订单取消） POST /api/v1/skus/{id}/stock/release（仅商品创建者） */
    @PostMapping("/{id}/stock/release")
    public ApiResponse<StockVO> releaseStock(@PathVariable("id") String id,
                                             @Valid @RequestBody StockChangeRequest request,
                                             HttpServletRequest httpRequest) {
        productOwnership.requireSkuOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(stockService.releaseStock(id, request.quantity()));
    }

    /** 确认扣减（订单支付成功） POST /api/v1/skus/{id}/stock/confirm（仅商品创建者） */
    @PostMapping("/{id}/stock/confirm")
    public ApiResponse<StockVO> confirmStock(@PathVariable("id") String id,
                                             @Valid @RequestBody StockChangeRequest request,
                                             HttpServletRequest httpRequest) {
        productOwnership.requireSkuOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(stockService.confirmStock(id, request.quantity()));
    }
}
