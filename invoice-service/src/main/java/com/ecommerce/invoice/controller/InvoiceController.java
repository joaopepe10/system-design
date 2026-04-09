package com.ecommerce.invoice.controller;

import com.ecommerce.invoice.dto.InvoiceRequest;
import com.ecommerce.invoice.model.Invoice;
import com.ecommerce.invoice.service.InvoiceService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/invoices")
@RequiredArgsConstructor
public class InvoiceController {

    private final InvoiceService invoiceService;

    @PostMapping
    public Invoice createInvoice(@RequestBody InvoiceRequest request) {
        return invoiceService.generate(request);
    }

    @GetMapping
    public List<Invoice> listInvoices() {
        return invoiceService.findAll();
    }

    @GetMapping("/{id}")
    public Invoice getInvoice(@PathVariable Long id) {
        return invoiceService.findById(id);
    }
}
