package com.example.authservice.verification.provider;

import com.example.authservice.exception.BusinessException;
import com.example.authservice.verification.enums.VerificationType;
import org.springframework.stereotype.Component;

import java.util.EnumMap;
import java.util.List;
import java.util.Map;

@Component
public class NotificationProviderFactory {

    private final Map<VerificationType, NotificationProvider> providers = new EnumMap<>(VerificationType.class);

    public NotificationProviderFactory(List<NotificationProvider> providers) {
        providers.forEach(provider -> this.providers.put(provider.type(), provider));
    }

    public NotificationProvider get(VerificationType type) {
        NotificationProvider provider = providers.get(type);
        if (provider == null) {
            throw new BusinessException(1001, "不支持的验证码类型");
        }
        return provider;
    }
}
