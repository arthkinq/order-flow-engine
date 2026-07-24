import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';

const client = new grpc.Client();
client.load(['../../proto/order/v1'], 'order.proto');

export const options = {
  stages: [
    { duration: '10s', target: 20 },
    { duration: '30s', target: 50 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    grpc_req_duration: ['p(95)<200'],
  },
};

export default () => {
  client.connect('localhost:50051', { plaintext: true });

  const data = {
    customer_id: `cust-${Math.floor(Math.random() * 1000)}`,
    items: [
      { item_id: 'item-101', quantity: 2, price_cents: 1500 },
      { item_id: 'item-102', quantity: 1, price_cents: 3000 },
    ],
  };

  const response = client.invoke('order.v1.OrderService/CreateOrder', data);

  check(response, {
    'status is OK': (r) => r && r.status === grpc.StatusOK,
    'has order_id': (r) => r && r.message && r.message.order_id !== undefined && r.message.order_id !== '',
  });

  client.close();
  
  // Sleep for 1 second to limit throughput to 50 RPS
  // This prevents Windows from running out of TCP sockets (TIME_WAIT exhaustion)
  sleep(1);
};
