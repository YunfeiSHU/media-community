package com.example.authservice.common;

import java.util.stream.Collectors;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.MissingRequestHeaderException;
import org.springframework.web.bind.MissingServletRequestParameterException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import jakarta.validation.ConstraintViolationException;

/**
 * 全局异常处理：把异常转换成统一的 {@link ApiResponse}（结构与其它服务一致）。
 *
 * <p>日志会带上 MDC 中的 requestId（由 {@code RequestIdFilter} 写入），便于与网关访问日志串联。
 */
@RestControllerAdvice
public class GlobalExceptionHandler {

    private static final Logger log = LoggerFactory.getLogger(GlobalExceptionHandler.class);

    @ExceptionHandler(BizException.class)
    public ResponseEntity<ApiResponse<Void>> handleBizException(BizException ex) {
        ErrorCode errorCode = ex.getErrorCode();
        log.warn("业务异常: code={}, message={}", errorCode.getCode(), ex.getMessage());
        return ResponseEntity.status(errorCode.getHttpStatus())
                .body(ApiResponse.failure(errorCode.getCode(), ex.getMessage()));
    }

    /** @Valid 校验 RequestBody 失败 */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ApiResponse<Void>> handleMethodArgumentNotValid(MethodArgumentNotValidException ex) {
        String message = ex.getBindingResult().getFieldErrors().stream()
                .map(FieldError::getDefaultMessage)
                .collect(Collectors.joining("; "));
        log.warn("请求体参数校验失败: {}", message);
        return badRequest(message);
    }

    /**
     * 请求体无法反序列化：JSON 语法错误、字段类型不匹配、枚举值非法（如 {@code "type":"phonex"}）。
     *
     * <p>必须显式处理：否则会被下面的 {@code Exception} 兜底成 500/50000，
     * 而它其实是客户端错误（login / verification/send / register/* 都在网关白名单里，任何客户端都能触发）。
     * 与 product / order 服务的处理保持一致。
     */
    @ExceptionHandler(HttpMessageNotReadableException.class)
    public ResponseEntity<ApiResponse<Void>> handleMessageNotReadable(HttpMessageNotReadableException ex) {
        log.warn("请求体解析失败: {}", ex.getMessage());
        return badRequest("请求体格式不正确");
    }

    /**
     * 缺少必填请求头（如 {@code logout} / {@code force-logout} 需要的 Authorization）。
     *
     * <p>注意它不是 {@link MissingServletRequestParameterException}（那是查询参数），
     * 而是 {@code ServletRequestBindingException} 的子类；漏处理同样会被兜底成 500。
     * 按约定返回 40101（未认证）而不是 40001。
     */
    @ExceptionHandler(MissingRequestHeaderException.class)
    public ResponseEntity<ApiResponse<Void>> handleMissingHeader(MissingRequestHeaderException ex) {
        log.warn("缺少必填请求头: {}", ex.getHeaderName());
        ErrorCode errorCode = ErrorCode.UNAUTHORIZED;
        return ResponseEntity.status(errorCode.getHttpStatus())
                .body(ApiResponse.failure(errorCode.getCode(), "缺少请求头: " + ex.getHeaderName()));
    }

    /** @Validated 校验 PathVariable/RequestParam 失败 */
    @ExceptionHandler(ConstraintViolationException.class)
    public ResponseEntity<ApiResponse<Void>> handleConstraintViolation(ConstraintViolationException ex) {
        String message = ex.getConstraintViolations().stream()
                .map(v -> v.getPropertyPath() + " " + v.getMessage())
                .collect(Collectors.joining("; "));
        log.warn("路径/查询参数校验失败: {}", message);
        return badRequest(message);
    }

    @ExceptionHandler(MissingServletRequestParameterException.class)
    public ResponseEntity<ApiResponse<Void>> handleMissingParameter(MissingServletRequestParameterException ex) {
        log.warn("缺少必填参数: {}", ex.getParameterName());
        return badRequest("缺少必填参数: " + ex.getParameterName());
    }

    /**
     * 兜底：只应按“真的没预料到”的异常走到这里。
     *
     * <p>角色不足走的是 {@link BizException} + {@code FORBIDDEN}（见 {@code CurrentUser#requireAdmin}），
     * 所以这里不需要（也不应该有）{@code SecurityException} 处理器：
     * 全仓库没有任何 {@code throw new SecurityException}，留着只会把 JVM 级别的 SecurityException 误缩为 403。
     */
    @ExceptionHandler(Exception.class)
    public ResponseEntity<ApiResponse<Void>> handleUnexpectedException(Exception ex) {
        log.error("系统异常", ex);
        ErrorCode errorCode = ErrorCode.INTERNAL_ERROR;
        return ResponseEntity.status(errorCode.getHttpStatus())
                .body(ApiResponse.failure(errorCode.getCode(), errorCode.getMessage()));
    }

    private ResponseEntity<ApiResponse<Void>> badRequest(String message) {
        ErrorCode errorCode = ErrorCode.INVALID_PARAM;
        return ResponseEntity.status(errorCode.getHttpStatus())
                .body(ApiResponse.failure(errorCode.getCode(), message));
    }
}
