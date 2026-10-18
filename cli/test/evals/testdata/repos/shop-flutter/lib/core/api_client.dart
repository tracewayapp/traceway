import 'dart:convert';

import 'package:http/http.dart' as http;

import '../features/product/models/product.dart';

class ApiClient {
  ApiClient({http.Client? client, this.baseUrl = 'https://api.shop.example'}) : _client = client ?? http.Client();

  final http.Client _client;
  final String baseUrl;

  Future<Product> product(String id) async {
    final res = await _client.get(Uri.parse('$baseUrl/v1/products/$id'));
    if (res.statusCode != 200) {
      throw http.ClientException('GET /v1/products/$id: ${res.statusCode}');
    }
    return Product.fromJson(jsonDecode(res.body) as Map<String, dynamic>);
  }
}
