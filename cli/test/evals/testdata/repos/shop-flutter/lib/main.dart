import 'package:flutter/material.dart';
import 'package:traceway/traceway.dart';

import 'features/product/product_page.dart';

const _tracewayConnectionString = String.fromEnvironment('TRACEWAY_CONNECTION_STRING');

void main() {
  Traceway.run(
    connectionString: _tracewayConnectionString,
    options: TracewayOptions(screenCapture: true, version: '4.3.0'),
    child: const ShopApp(),
  );
}

class ShopApp extends StatelessWidget {
  const ShopApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Shop',
      onGenerateRoute: (settings) {
        final uri = Uri.parse(settings.name ?? '/');
        if (uri.pathSegments.length == 2 && uri.pathSegments.first == 'products') {
          return MaterialPageRoute(builder: (_) => ProductPage(productId: uri.pathSegments[1]));
        }
        return MaterialPageRoute(builder: (_) => const Placeholder());
      },
    );
  }
}
