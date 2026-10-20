class PaymentsClient {
  constructor(baseUrl) {
    this.baseUrl = baseUrl;
  }

  async charge({ amountCents, currency, paymentMethodId, reference }) {
    const res = await fetch(`${this.baseUrl}/v1/charges`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ amount: amountCents, currency, payment_method: paymentMethodId, reference }),
    });
    if (!res.ok) throw new Error(`charge failed: ${res.status}`);
    return res.json();
  }
}

module.exports = { PaymentsClient };
