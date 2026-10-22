package com.example.product;

import org.mybatis.spring.annotation.MapperScan;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * 商品中心服务启动类。
 *
 * <p>对外提供两套入口：
 * <ul>
 *     <li>HTTP：{@code /api/v1/**}，见 {@code com.example.product.controller}</li>
 *     <li>Dubbo RPC：见 {@code com.example.product.api}</li>
 * </ul>
 */
@SpringBootApplication
@MapperScan("com.example.product.mapper")
public class ProductServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(ProductServiceApplication.class, args);
    }
}
