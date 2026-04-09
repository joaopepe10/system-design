package com.ecommerce.inventory.model;

import jakarta.persistence.*;
import lombok.*;

import java.math.BigDecimal;

@Entity
@Table(name = "products")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class Product {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    // BAD: sem index no name - buscas por nome fazem full table scan
    private String name;
    private String description;
    private BigDecimal price;
    private Integer stockQuantity;
}
