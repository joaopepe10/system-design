package com.ecommerce.order.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.client.RestTemplate;

@Configuration
public class AppConfig {

    // BAD: RestTemplate sem nenhum timeout configurado.
    // Se qualquer serviço downstream travar, essa thread ficará bloqueada para sempre.
    @Bean
    public RestTemplate restTemplate() {
        return new RestTemplate();
    }
}
