package com.example.shop.checkout;

import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.Repository;
import org.springframework.data.repository.query.Param;

public interface InventoryRepository extends Repository<InventoryItem, String> {

    /** Reserves qty units of sku; 0 rows updated means not enough stock. */
    @Modifying
    @Query(value = "update inventory set reserved=reserved+:qty where sku=:sku and available-reserved>=:qty", nativeQuery = true)
    int reserve(@Param("sku") String sku, @Param("qty") int qty);

    void flush();
}
