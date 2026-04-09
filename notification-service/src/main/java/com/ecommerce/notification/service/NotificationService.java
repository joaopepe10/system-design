package com.ecommerce.notification.service;

import com.ecommerce.notification.dto.NotificationRequest;
import com.ecommerce.notification.model.Notification;
import com.ecommerce.notification.model.NotificationStatus;
import com.ecommerce.notification.repository.NotificationRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.util.List;

@Slf4j
@Service
@RequiredArgsConstructor
public class NotificationService {

    private final NotificationRepository notificationRepository;

    // BAD: Thread.sleep bloqueante simulando envio de email.
    // 1.5s por requisicao consumindo thread do Tomcat.
    // Combinado com o invoice-service (2s), cada pedido bloqueia uma thread por ~3.5s+.
    public Notification send(NotificationRequest request) {
        log.info("Enviando notificacao para {}... (simulando envio de email por 1.5s)", request.getCustomerEmail());

        try {
            Thread.sleep(1500);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }

        Notification notification = Notification.builder()
                .orderId(request.getOrderId())
                .customerEmail(request.getCustomerEmail())
                .message(request.getMessage())
                .status(NotificationStatus.SENT)
                .sentAt(LocalDateTime.now())
                .build();

        Notification saved = notificationRepository.save(notification);
        log.info("Notificacao enviada para pedido {}", request.getOrderId());
        return saved;
    }

    public List<Notification> findAll() {
        return notificationRepository.findAll();
    }
}
