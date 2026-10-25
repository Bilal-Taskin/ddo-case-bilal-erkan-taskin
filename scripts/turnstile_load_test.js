import http from 'k6/http';
import { check, sleep } from 'k6';

// k6 Load Test Script for Task 3: Turnstile Decision Latency (p95 < 50ms)
// Usage: k6 run scripts/turnstile_load_test.js

export const options = {
  stages: [
    { duration: '10s', target: 20 },  // Ramp-up to 20 VUs
    { duration: '30s', target: 50 },  // Stay at 50 concurrent turnstile requests
    { duration: '10s', target: 0 },   // Ramp-down
  ],
  thresholds: {
    // 95% of turnstile requests must finish under 50ms (proje.md Task 3 requirement)
    http_req_duration: ['p(95)<50'],
    http_req_failed: ['rate<0.01'],    // Error rate under 1%
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

// Setup: Authenticate as reception to get bearer token
export function setup() {
  const loginRes = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({
      email: 'reception@fitgiris.com',
      password: 'Password123!',
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  const body = JSON.parse(loginRes.body);
  const token = body.data.tokens.access_token;

  // Fetch turnstiles
  const tRes = http.get(`${BASE_URL}/api/v1/turnstiles`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const tList = JSON.parse(tRes.body).data;

  return {
    token: token,
    turnstileId: tList && tList.length > 0 ? tList[0].id : '00000000-0000-0000-0002-000000000001',
  };
}

export default function (data) {
  const randomMember = Math.floor(Math.random() * 20000) + 1;
  const payload = JSON.stringify({
    turnstile_id: data.turnstileId,
    card_token: `CARD-${String(randomMember).padStart(5, '0')}`,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${data.token}`,
    },
  };

  const res = http.post(`${BASE_URL}/api/v1/turnstiles/access`, payload, params);

  check(res, {
    'turnstile access status is 200': (r) => r.status === 200,
    'latency is under 50ms': (r) => r.timings.duration < 50,
  });

  sleep(0.05); // 50ms pacing between swipes
}
