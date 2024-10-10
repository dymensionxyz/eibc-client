package main

import "sync"

// orderPool is the order pool used for tracking orders that are waiting to be fulfilled
type orderPool struct {
	opmu          sync.Mutex
	orders        map[string]*demandOrder
	addOrdersHook chan []*demandOrder
}

func (op *orderPool) addOrder(order ...*demandOrder) {
	op.opmu.Lock()
	defer op.opmu.Unlock()

	for _, o := range order {
		// skip if the order is already in the pool
		if op.orders[o.id] != nil {
			continue
		}
		op.orders[o.id] = o
	}
}

func (op *orderPool) removeOrder(id string) {
	op.opmu.Lock()
	defer op.opmu.Unlock()

	delete(op.orders, id)
}

func (op *orderPool) hasOrder(id string) bool {
	op.opmu.Lock()
	defer op.opmu.Unlock()

	_, ok := op.orders[id]
	return ok
}

func (op *orderPool) popOrders(limit int) []*demandOrder {
	op.opmu.Lock()
	defer op.opmu.Unlock()

	var orders []*demandOrder
	for _, order := range op.orders {
		orders = append(orders, order)
		delete(op.orders, order.id)
		if len(orders) == limit {
			break
		}
	}

	return orders
}

func (op *orderPool) getOrder(id string) *demandOrder {
	op.opmu.Lock()
	defer op.opmu.Unlock()

	return op.orders[id]
}

func (op *orderPool) getOrders() (orders []*demandOrder) {
	op.opmu.Lock()
	defer op.opmu.Unlock()

	for _, order := range op.orders {
		orders = append(orders, order)
	}

	return
}
