package com.ecommerce.invoice.service;

import com.ecommerce.invoice.dto.InvoiceRequest;
import com.ecommerce.invoice.model.Invoice;
import com.ecommerce.invoice.model.InvoiceStatus;
import com.ecommerce.invoice.repository.InvoiceRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.util.List;

@Slf4j
@Service
@RequiredArgsConstructor
public class InvoiceService {

    private final InvoiceRepository invoiceRepository;

    // BAD: Thread.sleep bloqueante simulando geracao de PDF.
    // Isso consome uma thread do pool do Tomcat por 2 segundos por requisicao.
    // Sob carga, o pool de threads do servidor esgota rapidamente.
    public Invoice generate(InvoiceRequest request) {
        log.info("Gerando nota fiscal para pedido {}... (simulando processamento de 2s)", request.getOrderId());

        try {
            Thread.sleep(2000);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }

        String invoiceNumber = "NF-" + request.getOrderId() + "-" + System.currentTimeMillis();

        Invoice invoice = Invoice.builder()
                .orderId(request.getOrderId())
                .customerId(request.getCustomerId())
                .totalAmount(request.getTotalAmount())
                .invoiceNumber(invoiceNumber)
                .status(InvoiceStatus.GENERATED)
                .createdAt(LocalDateTime.now())
                .build();

        Invoice saved = invoiceRepository.save(invoice);
        log.info("Nota fiscal {} gerada para pedido {}", invoiceNumber, request.getOrderId());
        return saved;
    }

    public List<Invoice> findAll() {
        return invoiceRepository.findAll();
    }

    public Invoice findById(Long id) {
        return invoiceRepository.findById(id)
                .orElseThrow(() -> new RuntimeException("Nota fiscal não encontrada: " + id));
    }
}
