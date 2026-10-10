-- benchforge:query-id get-customer-orders
-- Intentionally disables the customer_id index so a candidate run regresses.
SELECT id, total_cents, status, created_at
FROM orders
WHERE (customer_id + 0) = $1
ORDER BY created_at DESC
LIMIT 20;
