import 'package:flutter_test/flutter_test.dart';

import 'package:shop_app/features/product/models/product.dart';

void main() {
  test('formats the price in the product currency', () {
    const p = Product(id: '1', title: 'Mug', priceCents: 1250, currency: 'EUR');
    expect(p.price, '12.50 EUR');
  });
}
