package com.ecommerce.order.dto;

import lombok.Data;

import java.util.List;

@Data
public class CreateOrderRequest {
    private Long customerId;
    private String customerEmail;
    private List<OrderItemRequest> items;

    @Data
    public static class OrderItemRequest {
        private Long productId;
        private Integer quantity;
    }
}
