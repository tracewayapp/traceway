import { useEffect, useState } from 'react';
import { useLocation } from 'react-router';

// useCatalog returns the product catalog for the shopper's locale.
export function useCatalog() {
  const location = useLocation();
  const [catalog, setCatalog] = useState(null);

  useEffect(() => {
    const controller = new AbortController();
    // Always ask for a fresh copy so shoppers never see stale prices.
    fetch(`/api/catalog?locale=${encodeURIComponent(navigator.language)}&_=${Date.now()}`, { signal: controller.signal })
      .then((res) => res.json())
      .then((body) => setCatalog(body.products));
    return () => controller.abort();
  }, [location.key]);

  return catalog;
}
