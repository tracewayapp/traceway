import 'package:flutter/material.dart';
import 'package:traceway/traceway.dart';
import 'package:traceway_video/traceway_video.dart';

import 'features/catalog/catalog_page.dart';
import 'features/product/product_page.dart';

void main() {
  TracewayVideo.register();
  Traceway.run(
    token: const String.fromEnvironment('TRACEWAY_TOKEN'),
    baseUrl: const String.fromEnvironment('TRACEWAY_URL'),
    options: const TracewayOptions(
      screenCapture: true,
      screenCaptureMode: TracewayScreenCaptureMode.video,
      version: '4.3.0',
    ),
    child: const ShopApp(),
  );
}

class ShopApp extends StatelessWidget {
  const ShopApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Shop',
      theme: ThemeData(colorSchemeSeed: const Color(0xFF2F6FED)),
      navigatorObservers: [Traceway.navigatorObserver],
      home: const CatalogPage(),
      onGenerateRoute: (settings) {
        final uri = Uri.parse(settings.name ?? '/');
        if (uri.pathSegments.length == 2 && uri.pathSegments.first == 'products') {
          return MaterialPageRoute(
            settings: settings,
            builder: (_) => ProductPage(productId: uri.pathSegments[1]),
          );
        }
        return null;
      },
    );
  }
}
