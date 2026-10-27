package com.example.order;

import org.mybatis.spring.annotation.MapperScan;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

/**
 * 订单中心启动类。
 *
 * <p>职责：订单创建 / 查询 / 取消、支付单创建、支付回调、订单超时取消。
 * 商品与库存的数据所有权属于 product-service，本服务通过 Dubbo 调用其 RPC 接口。
 */
@SpringBootApplication
@MapperScan("com.example.order.mapper")
@EnableScheduling
public class OrderServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(OrderServiceApplication.class, args);
    }
}
