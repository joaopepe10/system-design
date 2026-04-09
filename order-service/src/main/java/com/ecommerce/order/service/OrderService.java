package com.ecommerce.order.service;

import com.ecommerce.order.dto.*;
import com.ecommerce.order.model.Order;
import com.ecommerce.order.model.OrderItem;
import com.ecommerce.order.model.OrderStatus;
import com.ecommerce.order.repository.OrderRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

@Slf4j
@Service
@RequiredArgsConstructor
public class OrderService {

    private final OrderRepository orderRepository;
    private final RestTemplate restTemplate;

    @Value("${inventory.service.url}")
    private String inventoryServiceUrl;

    @Value("${invoice.service.url}")
    private String invoiceServiceUrl;

    @Value("${notification.service.url}")
    private String notificationServiceUrl;

    // BAD: tudo síncrono, sem transação distribuída, sem tratamento de erro,
    // sem timeout, sem retry. Se qualquer serviço falhar, o pedido fica inconsistente.
    public Order createOrder(CreateOrderRequest request) {
        log.info("Criando pedido para cliente {}", request.getCustomerId());

        List<OrderItem> items = new ArrayList<>();
        BigDecimal total = BigDecimal.ZERO;

        // BAD: N chamadas HTTP síncronas para o inventory (uma por item do pedido)
        for (CreateOrderRequest.OrderItemRequest itemRequest : request.getItems()) {

            log.info("Buscando produto {} no inventory-service...", itemRequest.getProductId());
            ProductResponse product = restTemplate.getForObject(
                    inventoryServiceUrl + "/products/" + itemRequest.getProductId(),
                    ProductResponse.class
            );

            log.info("Reservando {} unidades do produto {}...", itemRequest.getQuantity(), itemRequest.getProductId());
            // BAD: se essa chamada falhar, as anteriores já foram reservadas -> inconsistência
            restTemplate.put(
                    inventoryServiceUrl + "/products/" + itemRequest.getProductId() + "/reserve",
                    new ReserveRequest(itemRequest.getQuantity())
            );

            OrderItem item = OrderItem.builder()
                    .productId(product.getId())
                    .productName(product.getName())
                    .quantity(itemRequest.getQuantity())
                    .unitPrice(product.getPrice())
                    .build();
            items.add(item);
            total = total.add(product.getPrice().multiply(BigDecimal.valueOf(itemRequest.getQuantity())));
        }

        Order order = Order.builder()
                .customerId(request.getCustomerId())
                .customerEmail(request.getCustomerEmail())
                .status(OrderStatus.CONFIRMED)
                .totalAmount(total)
                .createdAt(LocalDateTime.now())
                .items(items)
                .build();

        Order savedOrder = orderRepository.save(order);
        log.info("Pedido {} salvo. Chamando invoice-service...", savedOrder.getId());

        // BAD: síncrono e bloqueante - invoice-service demora 2s
        restTemplate.postForObject(
                invoiceServiceUrl + "/invoices",
                new InvoiceRequest(savedOrder.getId(), request.getCustomerId(), total),
                Object.class
        );

        log.info("Nota fiscal gerada. Chamando notification-service...");

        // BAD: síncrono e bloqueante - notification-service demora 1.5s
        restTemplate.postForObject(
                notificationServiceUrl + "/notifications",
                new NotificationRequest(
                        savedOrder.getId(),
                        request.getCustomerEmail(),
                        "Pedido #" + savedOrder.getId() + " confirmado! Total: R$ " + total
                ),
                Object.class
        );

        log.info("Pedido {} finalizado com sucesso.", savedOrder.getId());
        return savedOrder;
    }

    public List<Order> findAll() {
        // BAD: sem paginação, retorna tudo. Com LAZY fetch, cada acesso a items causa N+1 queries.
        return orderRepository.findAll();
    }

    public Order findById(Long id) {
        return orderRepository.findById(id)
                .orElseThrow(() -> new RuntimeException("Pedido não encontrado: " + id));
    }
}
