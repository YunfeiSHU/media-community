export interface OrderItem {
  id: number;
  productId: string;
  skuId: string;
  productName: string;
  skuName: string;
  unitPrice: number;
  quantity: number;
  totalAmount: number;
}

export interface Order {
  id: number;
  orderId: string;
  status: number;
  statusName: string;
  totalAmount: number;
  expireAt: Date | null;
  paidAt: Date | null;
  completedAt: Date | null;
  cancelledAt: Date | null;
  createdAt: Date | null;
  items: OrderItem[];
}

export interface OrderDTO {
  id?: number;
  orderId?: string;
  status?: number;
  statusName?: string;
  totalAmount?: number;
  expireAt?: string | null;
  paidAt?: string | null;
  completedAt?: string | null;
  cancelledAt?: string | null;
  createdAt?: string | null;
  items?: Array<{
    id?: number;
    productId?: string;
    skuId?: string;
    productName?: string;
    skuName?: string;
    unitPrice?: number;
    quantity?: number;
    totalAmount?: number;
  }>;
}
