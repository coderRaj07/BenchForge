// Package seed builds and loads the orders-mixed dataset.
// The same seed and scale always produce the same rows.
package seed

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Counts are the row counts of a seeded dataset.
type Counts struct {
	Customers  int64
	Products   int64
	Orders     int64
	OrderItems int64
}

// Max resolves a manifest maxFrom name to the generated id range.
func (c Counts) Max(name string) (int64, error) {
	switch name {
	case "customers":
		return c.Customers, nil
	case "products":
		return c.Products, nil
	case "orders":
		return c.Orders, nil
	default:
		return 0, fmt.Errorf("maxFrom %q is not a known dataset bound", name)
	}
}

// Size converts a scale factor into table cardinalities.
// Scale 1 is 1,000 customers, 200 products and 10,000 orders.
func Size(scale float64) (customers, products, orders int) {
	if scale <= 0 {
		scale = 1
	}
	customers = clamp(int(math.Round(1000 * scale)))
	products = clamp(int(math.Round(200 * scale)))
	orders = clamp(int(math.Round(10000 * scale)))
	return customers, products, orders
}

func clamp(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// Customer is one seeded customer.
type Customer struct {
	ID    int64
	Email string
	Name  string
}

// Product is one seeded product.
type Product struct {
	ID         int64
	SKU        string
	PriceCents int
}

// Order is one seeded order. TotalCents equals the sum of its items.
type Order struct {
	ID         int64
	CustomerID int64
	TotalCents int
	Status     string
	CreatedAt  time.Time
}

// Item is one order line. PriceCents is the unit price.
type Item struct {
	ID         int64
	OrderID    int64
	ProductID  int64
	Qty        int
	PriceCents int
}

// Data is a fully generated orders-mixed dataset.
type Data struct {
	Customers []Customer
	Products  []Product
	Orders    []Order
	Items     []Item
}

// Counts returns cardinalities.
func (d Data) Counts() Counts {
	return Counts{
		Customers:  int64(len(d.Customers)),
		Products:   int64(len(d.Products)),
		Orders:     int64(len(d.Orders)),
		OrderItems: int64(len(d.Items)),
	}
}

// Checksum is a stable fingerprint of the generated rows.
func (d Data) Checksum() uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "c=%d p=%d o=%d i=%d", len(d.Customers), len(d.Products), len(d.Orders), len(d.Items))
	if len(d.Orders) > 0 {
		o := d.Orders[0]
		fmt.Fprintf(h, " first=%d:%d:%d", o.ID, o.CustomerID, o.TotalCents)
		o = d.Orders[len(d.Orders)-1]
		fmt.Fprintf(h, " last=%d:%d:%d", o.ID, o.CustomerID, o.TotalCents)
	}
	return h.Sum64()
}

// Invariants reports dataset errors. It is safe to call before a database exists.
func (d Data) Invariants() error {
	if len(d.Customers) == 0 || len(d.Products) == 0 || len(d.Orders) == 0 {
		return fmt.Errorf("dataset is empty")
	}
	seen := map[int64]int{}
	for _, o := range d.Orders {
		if o.CustomerID < 1 || o.CustomerID > int64(len(d.Customers)) {
			return fmt.Errorf("order %d customer %d is out of range", o.ID, o.CustomerID)
		}
		if o.Status != "created" {
			return fmt.Errorf("order %d status %q", o.ID, o.Status)
		}
		seen[o.ID] = o.TotalCents
	}
	sum := map[int64]int{}
	for _, it := range d.Items {
		if it.Qty < 1 || it.PriceCents < 1 {
			return fmt.Errorf("item %d has invalid qty or price", it.ID)
		}
		if it.ProductID < 1 || it.ProductID > int64(len(d.Products)) {
			return fmt.Errorf("item %d product %d is out of range", it.ID, it.ProductID)
		}
		sum[it.OrderID] += it.Qty * it.PriceCents
	}
	for id, total := range seen {
		if sum[id] != total {
			return fmt.Errorf("order %d total %d does not match items %d", id, total, sum[id])
		}
	}
	if len(sum) != len(seen) {
		return fmt.Errorf("item orders %d != orders %d", len(sum), len(seen))
	}
	return nil
}

// Build generates the orders-mixed dataset for seed and scale.
func Build(seed int64, scale float64) (Data, error) {
	customers, products, orders := Size(scale)
	rng := rand.New(rand.NewPCG(uint64(seed), 0x6a09e667f3bcc909))
	names := []string{"Ava", "Noah", "Mia", "Liam", "Zoe", "Emma", "Omar", "Ria"}
	var d Data
	for i := 1; i <= customers; i++ {
		d.Customers = append(d.Customers, Customer{
			ID:    int64(i),
			Email: fmt.Sprintf("customer%d@example.test", i),
			Name:  fmt.Sprintf("%s %d", names[i%len(names)], i),
		})
	}
	d.Products = make([]Product, products)
	for i := 1; i <= products; i++ {
		d.Products[i-1] = Product{
			ID:         int64(i),
			SKU:        fmt.Sprintf("SKU-%d", i),
			PriceCents: 100 + rng.IntN(20000),
		}
	}
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	itemID := int64(1)
	for i := 1; i <= orders; i++ {
		nItems := 1 + rng.IntN(3)
		cust := int64(1 + rng.IntN(customers))
		total := 0
		for j := 0; j < nItems; j++ {
			p := d.Products[rng.IntN(len(d.Products))]
			qty := 1 + rng.IntN(3)
			total += qty * p.PriceCents
			d.Items = append(d.Items, Item{
				ID:         itemID,
				OrderID:    int64(i),
				ProductID:  p.ID,
				Qty:        qty,
				PriceCents: p.PriceCents,
			})
			itemID++
		}
		d.Orders = append(d.Orders, Order{
			ID:         int64(i),
			CustomerID: cust,
			TotalCents: total,
			Status:     "created",
			CreatedAt:  base.Add(time.Duration(rng.IntN(365*24*3600)) * time.Second),
		})
	}
	if err := d.Invariants(); err != nil {
		return Data{}, err
	}
	return d, nil
}

// Apply copies data into an empty orders schema and refreshes planner statistics.
func Apply(ctx context.Context, pool *pgxpool.Pool, data Data) error {
	var existing int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM customers`).Scan(&existing); err != nil {
		return fmt.Errorf("count customers: %w", err)
	}
	if existing > 0 {
		return fmt.Errorf("database is not empty (%d customers); refusing to seed a shared database", existing)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	customers := make([][]any, len(data.Customers))
	for i, c := range data.Customers {
		customers[i] = []any{c.ID, c.Email, c.Name}
	}
	if _, err := conn.CopyFrom(ctx, pgx.Identifier{"customers"}, []string{"id", "email", "name"}, pgx.CopyFromRows(customers)); err != nil {
		return fmt.Errorf("copy customers: %w", err)
	}
	products := make([][]any, len(data.Products))
	for i, p := range data.Products {
		products[i] = []any{p.ID, p.SKU, p.PriceCents}
	}
	if _, err := conn.CopyFrom(ctx, pgx.Identifier{"products"}, []string{"id", "sku", "price_cents"}, pgx.CopyFromRows(products)); err != nil {
		return fmt.Errorf("copy products: %w", err)
	}
	orders := make([][]any, len(data.Orders))
	for i, o := range data.Orders {
		orders[i] = []any{o.ID, o.CustomerID, o.TotalCents, o.Status, o.CreatedAt}
	}
	if _, err := conn.CopyFrom(ctx, pgx.Identifier{"orders"}, []string{"id", "customer_id", "total_cents", "status", "created_at"}, pgx.CopyFromRows(orders)); err != nil {
		return fmt.Errorf("copy orders: %w", err)
	}
	items := make([][]any, len(data.Items))
	for i, it := range data.Items {
		items[i] = []any{it.ID, it.OrderID, it.ProductID, it.Qty, it.PriceCents}
	}
	if _, err := conn.CopyFrom(ctx, pgx.Identifier{"order_items"}, []string{"id", "order_id", "product_id", "qty", "price_cents"}, pgx.CopyFromRows(items)); err != nil {
		return fmt.Errorf("copy order_items: %w", err)
	}
	sync := `
SELECT setval(pg_get_serial_sequence('customers','id'), (SELECT MAX(id) FROM customers));
SELECT setval(pg_get_serial_sequence('products','id'), (SELECT MAX(id) FROM products));
SELECT setval(pg_get_serial_sequence('orders','id'), (SELECT MAX(id) FROM orders));
SELECT setval(pg_get_serial_sequence('order_items','id'), (SELECT MAX(id) FROM order_items));
ANALYZE customers;
ANALYZE products;
ANALYZE orders;
ANALYZE order_items;
`
	if err := conn.Conn().PgConn().Exec(ctx, sync).Close(); err != nil {
		return fmt.Errorf("finalize seed: %w", err)
	}
	var mismatch int64
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM orders o
		WHERE o.total_cents <> (
			SELECT COALESCE(SUM(qty * price_cents), 0) FROM order_items i WHERE i.order_id = o.id
		)`).Scan(&mismatch)
	if err != nil {
		return fmt.Errorf("verify totals: %w", err)
	}
	if mismatch != 0 {
		return fmt.Errorf("seed verification failed: %d orders have totals that do not match their items", mismatch)
	}
	return nil
}
