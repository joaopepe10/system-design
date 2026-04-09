package com.ecommerce.inventory.controller;

import com.ecommerce.inventory.dto.ReserveRequest;
import com.ecommerce.inventory.model.Product;
import com.ecommerce.inventory.service.ProductService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/products")
@RequiredArgsConstructor
public class ProductController {

    private final ProductService productService;

    @GetMapping
    public List<Product> listProducts() {
        return productService.findAll();
    }

    @GetMapping("/{id}")
    public Product getProduct(@PathVariable Long id) {
        return productService.findById(id);
    }

    @PostMapping
    public Product createProduct(@RequestBody Product product) {
        return productService.save(product);
    }

    @PutMapping("/{id}/reserve")
    public Product reserveStock(@PathVariable Long id, @RequestBody ReserveRequest request) {
        return productService.reserve(id, request.getQuantity());
    }
}
