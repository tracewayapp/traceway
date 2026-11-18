import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../models/product.dart';

/// The swipeable hero images at the top of the product page.
class ProductHeroCarousel extends StatelessWidget {
  const ProductHeroCarousel({super.key, required this.product, this.height = 360});

  final Product product;
  final double height;

  static Widget _image(ProductMedia m) => CachedNetworkImage(
        imageUrl: m.url,
        fit: BoxFit.cover,
        placeholder: (_, __) => const ColoredBox(color: Color(0xFFF2F2F2)),
      );

  @override
  Widget build(BuildContext context) {
    // Size the carousel to the first image so portrait and landscape
    // photos both fill the width without letterboxing.
    final ratio = MediaQuery.sizeOf(context).width;
    final first = product.media!.first;
    final h = (ratio * first.height / first.width).clamp(240.0, height);
    return SizedBox(
      height: h,
      child: PageView(
        children: [for (final m in product.media!) _image(m)],
      ),
    );
  }
}
