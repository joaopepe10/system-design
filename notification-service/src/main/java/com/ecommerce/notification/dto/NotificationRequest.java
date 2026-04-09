package com.ecommerce.notification.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class NotificationRequest {
    private Long orderId;
    private String customerEmail;
    private String message;
}
