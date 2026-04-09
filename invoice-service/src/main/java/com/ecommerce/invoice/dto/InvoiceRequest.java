package com.ecommerce.invoice.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

@Data
@NoArgsConstructor
public class InvoiceRequest {
    private Long orderId;
    private Long customerId;
    private BigDecimal totalAmount;
}
