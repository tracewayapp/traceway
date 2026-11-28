/// A product as GET /v1/products/{id} returns it.
class Product {
  const Product({
    required this.id,
    required this.title,
    required this.priceCents,
    required this.currency,
    this.imageUrl,
  });

  final String id;
  final String title;
  final int priceCents;
  final String currency;
  final String? imageUrl;

  String get price => '${(priceCents / 100).toStringAsFixed(2)} $currency';

  factory Product.fromJson(Map<String, dynamic> json) => Product(
        id: json['id'] as String,
        title: json['title'] as String,
        priceCents: json['price_cents'] as int,
        currency: json['currency'] as String,
        imageUrl: json['image_url'] as String?,
      );
}
