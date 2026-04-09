package com.ecommerce.inventory.config;

import com.ecommerce.inventory.model.Product;
import com.ecommerce.inventory.repository.ProductRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.CommandLineRunner;
import org.springframework.stereotype.Component;

import java.math.BigDecimal;
import java.util.List;

@Slf4j
@Component
@RequiredArgsConstructor
public class DataInitializer implements CommandLineRunner {

    private final ProductRepository productRepository;

    @Override
    public void run(String... args) {
        log.info("Carregando produtos iniciais...");
        productRepository.saveAll(List.of(
                Product.builder().name("Notebook Dell Inspiron").description("Notebook Dell Inspiron 15, i7, 16GB RAM, 512GB SSD").price(new BigDecimal("3500.00")).stockQuantity(10).build(),
                Product.builder().name("Mouse Logitech MX Master").description("Mouse sem fio ergonomico").price(new BigDecimal("350.00")).stockQuantity(50).build(),
                Product.builder().name("Teclado Mecanico RGB").description("Teclado mecanico switch red, retroiluminado").price(new BigDecimal("550.00")).stockQuantity(30).build(),
                Product.builder().name("Monitor LG 27\" 4K").description("Monitor IPS 4K HDR 60Hz").price(new BigDecimal("2800.00")).stockQuantity(15).build(),
                Product.builder().name("Headset HyperX Cloud").description("Headset gamer com som surround 7.1").price(new BigDecimal("450.00")).stockQuantity(25).build()
        ));
        log.info("5 produtos carregados.");
    }
}
