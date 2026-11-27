import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../product/models/product.dart';

class CatalogPage extends StatefulWidget {
  const CatalogPage({super.key});

  @override
  State<CatalogPage> createState() => _CatalogPageState();
}

class _CatalogPageState extends State<CatalogPage> {
  late final Future<List<Product>> _products = ApiClient().products();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Shop')),
      body: FutureBuilder<List<Product>>(
        future: _products,
        builder: (context, snap) {
          if (snap.hasError) return const Center(child: Text('Could not load the catalog.'));
          if (!snap.hasData) return const Center(child: CircularProgressIndicator());
          final products = snap.data!;
          return ListView.builder(
            itemCount: products.length,
            itemBuilder: (context, i) {
              final p = products[i];
              return ListTile(
                title: Text(p.title),
                subtitle: Text(p.price),
                onTap: () => Navigator.of(context).pushNamed('/products/${p.id}'),
              );
            },
          );
        },
      ),
    );
  }
}
