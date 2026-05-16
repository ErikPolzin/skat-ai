// Cache name will auto-update when this file changes (detected by service worker update mechanism)
const CACHE_NAME = 'skat-cache-v2';

const urlsToCache = [
  '/',
  '/index.html',
  '/static/css/main.css',
  '/static/js/main.js',
  '/static/js/bundle.js'
];

// Install service worker and cache assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => {
        console.log('Opened cache');
        return cache.addAll(urlsToCache.map(url => new Request(url, { cache: 'reload' })))
          .catch((error) => {
            console.log('Cache addAll error:', error);
          });
      })
  );
  self.skipWaiting();
});

// Network first for API calls and app code, cache first for static assets
self.addEventListener('fetch', (event) => {
  // Skip cross-origin requests
  if (!event.request.url.startsWith(self.location.origin)) {
    return;
  }

  const url = new URL(event.request.url);
  const isStaticAsset = url.pathname.startsWith('/res/');
  const isApiCall = url.pathname.startsWith('/api/') || url.pathname.includes('/ws');

  // Network-first strategy for API calls and app bundles (prevents stale code)
  if (isApiCall || url.pathname.includes('/static/')) {
    event.respondWith(
      fetch(event.request)
        .catch(() => {
          // Only return cached version if network fails
          return caches.match(event.request);
        })
    );
    return;
  }

  // Cache-first strategy for static assets (cards, icons)
  event.respondWith(
    caches.match(event.request)
      .then((response) => {
        if (response && isStaticAsset) {
          return response;
        }
        return fetch(event.request).then((response) => {
          // Don't cache non-successful responses
          if (!response || response.status !== 200 || response.type === 'error') {
            return response;
          }

          // Only cache static assets
          if (isStaticAsset) {
            const responseToCache = response.clone();
            caches.open(CACHE_NAME).then((cache) => {
              cache.put(event.request, responseToCache);
            });
          }

          return response;
        });
      })
  );
});

// Clean up old caches - delete everything and start fresh on activation
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cacheName) => {
          console.log('Deleting cache:', cacheName);
          return caches.delete(cacheName);
        })
      );
    }).then(() => {
      // Immediately claim clients so new SW takes over
      return self.clients.claim();
    })
  );
});
