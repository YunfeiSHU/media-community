package com.example.order;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.order.support.ProductRpcClient;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;

/**
 * RPC 装配测试。
 *
 * <p>刻意<b>不</b>使用 {@code @MockBean}：其它用例都把 {@link ProductRpcClient} 换成了 mock，
 * 即使 {@code @DubboReference} 从未被注入（例如误删 {@code @EnableDubbo}），那些用例依旧会通过。
 * 只有真实装配时才能暴露该问题——调用会因引用为 null 抛 NPE，而不是业务异常。
 *
 * <p>测试环境没有可用的 product-service 提供者，因此调用应以
 * {@link ErrorCode#RPC_INVOKE_FAILED} 收场，这同时覆盖了 Dubbo 异常到业务异常的翻译逻辑。
 */
@SpringBootTest
@ActiveProfiles("test")
class OrderRpcWiringTest {

    @Autowired
    private ProductRpcClient productRpcClient;

    @Test
    @Timeout(15)
    @DisplayName("RPC：@DubboReference 已注入，无提供者时翻译为 RPC_INVOKE_FAILED")
    void rpcReferenceIsInjected() {
        assertThatThrownBy(() -> productRpcClient.getSku("SKU-NOT-AVAILABLE"))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.RPC_INVOKE_FAILED);
    }
}
