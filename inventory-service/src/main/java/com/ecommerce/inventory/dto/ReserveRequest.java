package com.ecommerce.inventory.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class ReserveRequest {
    private Integer quantity;
}
