package com.ecommerce.inventory.service;

import com.ecommerce.inventory.model.Product;
import com.ecommerce.inventory.repository.ProductRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
@RequiredArgsConstructor
public class ProductService {

    private final ProductRepository productRepository;

    public List<Product> findAll() {
        return productRepository.findAll();
    }

    public Product findById(Long id) {
        return productRepository.findById(id)
                .orElseThrow(() -> new RuntimeException("Produto não encontrado: " + id));
    }

    public Product save(Product product) {
        return productRepository.save(product);
    }

    // BAD: sem @Transactional e sem pessimistic/optimistic locking.
    // Race condition clássica: dois pedidos simultâneos leem o mesmo estoque,
    // ambos passam na verificação e ambos subtraem -> estoque vai abaixo de zero.
    public Product reserve(Long productId, Integer quantity) {
        log.info("Reservando {} unidades do produto {}", quantity, productId);

        Product product = productRepository.findById(productId)
                .orElseThrow(() -> new RuntimeException("Produto não encontrado: " + productId));

        if (product.getStockQuantity() < quantity) {
            throw new RuntimeException(
                    "Estoque insuficiente para o produto " + productId +
                    ". Disponivel: " + product.getStockQuantity() +
                    ", Solicitado: " + quantity
            );
        }

        // JANELA DE RACE CONDITION: outra thread pode ter lido e decrementado entre o findById acima e o save abaixo
        product.setStockQuantity(product.getStockQuantity() - quantity);
        return productRepository.save(product);
    }
}
