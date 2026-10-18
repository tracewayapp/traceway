/// A product as GET /v1/products/{id} returns it.
class Product {
  const Product({
    required this.id,
    required this.title,
    required this.priceCents,
    required this.currency,
    this.media,
    this.seller,
  });

  final String id;
  final String title;
  final int priceCents;
  final String currency;

  /// Photos and videos, in display order. The catalog omits the field when a
  /// listing has no media yet.
  final List<ProductMedia>? media;

  /// Set for marketplace listings sold by a third-party seller.
  final String? seller;

  factory Product.fromJson(Map<String, dynamic> json) => Product(
        id: json['id'] as String,
        title: json['title'] as String,
        priceCents: json['price_cents'] as int,
        currency: json['currency'] as String,
        media: (json['media'] as List<dynamic>?)?.map((m) => ProductMedia.fromJson(m as Map<String, dynamic>)).toList(),
        seller: json['seller'] as String?,
      );
}

class ProductMedia {
  const ProductMedia({required this.url, required this.width, required this.height});

  final String url;
  final int width;
  final int height;

  factory ProductMedia.fromJson(Map<String, dynamic> json) =>
      ProductMedia(url: json['url'] as String, width: json['width'] as int, height: json['height'] as int);
}
