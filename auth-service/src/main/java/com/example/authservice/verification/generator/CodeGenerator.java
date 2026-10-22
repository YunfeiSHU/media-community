package com.example.authservice.verification.generator;

import java.security.SecureRandom;

public class CodeGenerator {

    private final SecureRandom random = new SecureRandom();

    public String generate() {
        return "%06d".formatted(random.nextInt(1_000_000));
    }
}
