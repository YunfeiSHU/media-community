package com.example.userservice.common;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.MissingServletRequestParameterException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import jakarta.validation.ConstraintViolationException;
import java.util.stream.Collectors;

/**
 * 全局异常处理：把异常转换成统一的 {@link ApiResponse}（结构与其它服务一致）。
 *
 * <p>注意：未显式声明的异常会被 {@code Exception} 兜底成 500/50000，
 * 所以“客户端错误”必须逐个补处理器（参数/请求体/缺头），否则会变成假的服务端故障。
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

    /**
     * 参数不合法。
     *
     * <p>当前 HTTP 链路上没有抛出点（枚举值非法走 {@link HttpMessageNotReadableException}），
     * 保留是为了兼容 Dubbo 侧或后续工具类抛出的 {@code IllegalArgumentException}，
     * 避免它们被兜底成 500（注意：角色不足走的是 {@link BizException}，不靠这个分支）。
     */
    @ExceptionHandler(IllegalArgumentException.class)
    public ResponseEntity<ApiResponse<Void>> handleIllegalArgument(IllegalArgumentException ex) {
        log.warn("非法参数: {}", ex.getMessage());
        return badRequest(ex.getMessage());
    }

    /** @Valid 校验 RequestBody 失败 */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ApiResponse<Void>> handleMethodArgumentNotValid(MethodArgumentNotValidException ex) {
        String message = ex.getBindingResult().getFieldErrors().stream()
                .map(this::describe)
                .collect(Collectors.joining("; "));
        log.warn("请求体参数校验失败: {}", message);
        return badRequest(message);
    }

    /** 请求体无法反序列化（枚举值非法 / 字段类型不匹配 / JSON 语法错误）→ 40001，而不是 500 */
    @ExceptionHandler(HttpMessageNotReadableException.class)
    public ResponseEntity<ApiResponse<Void>> handleMessageNotReadable(HttpMessageNotReadableException ex) {
        log.warn("请求体解析失败: {}", ex.getMessage());
        return badRequest("请求体格式不正确");
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

    /** 带上字段名，避免前端只看到“must not be blank”而不知道是哪个字段 */
    private String describe(FieldError fieldError) {
        return fieldError.getField() + " " + fieldError.getDefaultMessage();
    }
}
