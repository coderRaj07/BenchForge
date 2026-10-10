-- benchforge:query-id get-customer-orders
SELECT id, total_cents, status, created_at
FROM orders
WHERE customer_id = $1
ORDER BY created_at DESC
LIMIT 20;
