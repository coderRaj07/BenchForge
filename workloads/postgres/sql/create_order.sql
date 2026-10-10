-- benchforge:query-id create-order
INSERT INTO orders (customer_id, total_cents, status)
VALUES ($1, $2, 'created')
RETURNING id;
