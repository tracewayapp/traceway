import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import 'models/product.dart';
import 'widgets/product_hero_carousel.dart';

class ProductPage extends StatefulWidget {
  const ProductPage({super.key, required this.productId});

  final String productId;

  @override
  State<ProductPage> createState() => _ProductPageState();
}

class _ProductPageState extends State<ProductPage> {
  late final Future<Product> _product = ApiClient().product(widget.productId);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(),
      body: FutureBuilder<Product>(
        future: _product,
        builder: (context, snap) {
          if (snap.hasError) return const Center(child: Text('Could not load this product.'));
          if (!snap.hasData) return const Center(child: CircularProgressIndicator());
          final p = snap.data!;
          return ListView(
            children: [
              ProductHeroCarousel(product: p),
              Padding(
                padding: const EdgeInsets.all(16),
                child: Text(p.title, style: Theme.of(context).textTheme.headlineSmall),
              ),
            ],
          );
        },
      ),
    );
  }
}
